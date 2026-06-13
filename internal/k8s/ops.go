package k8s

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	sigsyaml "sigs.k8s.io/yaml"
)

// resourceClient returns the dynamic client scoped to the resource and, when
// applicable, namespace.
func (c *Client) resourceClient(res ResourceInfo, namespace string) dynamic.ResourceInterface {
	nr := c.dynamic.Resource(res.GVR())
	if res.Namespaced && namespace != "" {
		return nr.Namespace(namespace)
	}
	return nr
}

// GetYAML fetches a single object and renders it as YAML with managedFields
// stripped for readability. When decodeSecrets is set and the object is a
// Secret, its base64 data values are decoded for viewing (callers that intend
// to edit pass false so the round-trip stays valid base64).
func (c *Client) GetYAML(ctx context.Context, res ResourceInfo, namespace, name string, decodeSecrets bool) (string, error) {
	obj, err := c.resourceClient(res, namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
	if decodeSecrets && res.Group == "" && res.Kind == "Secret" {
		decodeSecretData(obj.Object)
	}
	b, err := sigsyaml.Marshal(obj.Object)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// GetObject fetches a single object as unstructured JSON with noisy managed
// fields stripped. Callers render the object without changing cluster data.
func (c *Client) GetObject(ctx context.Context, res ResourceInfo, namespace, name string) (map[string]interface{}, error) {
	obj, err := c.resourceClient(res, namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
	return obj.Object, nil
}

// decodeSecretData replaces base64 data values with their decoded text in place
// (for display). Binary values that are not valid UTF-8 are left as base64.
func decodeSecretData(obj map[string]interface{}) {
	data, ok := obj["data"].(map[string]interface{})
	if !ok {
		return
	}
	for k, v := range data {
		s, ok := v.(string)
		if !ok {
			continue
		}
		dec, err := base64.StdEncoding.DecodeString(s)
		if err == nil && utf8.Valid(dec) {
			data[k] = string(dec)
		}
	}
}

// RolloutRestart triggers a rolling restart of a workload by stamping the pod
// template with a restartedAt annotation, the same mechanism kubectl uses.
func (c *Client) RolloutRestart(ctx context.Context, res ResourceInfo, namespace, name string) error {
	ts := time.Now().Format(time.RFC3339)
	patch := []byte(fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":%q}}}}}`, ts))
	_, err := c.resourceClient(res, namespace).Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	return err
}

// TriggerCronJob creates a one-off Job from a CronJob's jobTemplate, similar to
// `kubectl create job --from=cronjob/<name>`.
func (c *Client) TriggerCronJob(ctx context.Context, namespace, name string) (string, error) {
	cj, err := c.clientset.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	annotations := cloneStringMap(cj.Spec.JobTemplate.Annotations)
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations["cronjob.kubernetes.io/instantiate"] = "manual"
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: cronJobRunPrefix(name),
			Namespace:    namespace,
			Labels:       cloneStringMap(cj.Spec.JobTemplate.Labels),
			Annotations:  annotations,
		},
		Spec: cj.Spec.JobTemplate.Spec,
	}
	created, err := c.clientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

func cronJobRunPrefix(name string) string {
	const suffix = "-manual-"
	const maxPrefix = 52 // leave room for the API server's generated suffix
	if len(name)+len(suffix) > maxPrefix {
		name = strings.TrimRight(name[:maxPrefix-len(suffix)], "-")
	}
	return name + suffix
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Delete removes a single object.
func (c *Client) Delete(ctx context.Context, res ResourceInfo, namespace, name string) error {
	return c.resourceClient(res, namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// Apply replaces an object from edited YAML (kubectl edit semantics: an
// optimistic update guarded by resourceVersion). It rejects edits that change
// the object's kind or name, which would otherwise be sent to the wrong
// endpoint or target a different object.
func (c *Client) Apply(ctx context.Context, res ResourceInfo, namespace, name string, yamlBytes []byte) error {
	jsonBytes, err := sigsyaml.YAMLToJSON(yamlBytes)
	if err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	obj := &unstructured.Unstructured{}
	if err := obj.UnmarshalJSON(jsonBytes); err != nil {
		return fmt.Errorf("decode object: %w", err)
	}

	gvk := obj.GroupVersionKind()
	if gvk.Kind != res.Kind || gvk.Group != res.Group {
		return fmt.Errorf("edited kind %q does not match %s; change rejected", gvk.Kind, res.Kind)
	}
	if name != "" && obj.GetName() != name {
		return fmt.Errorf("edited name %q does not match %q; change rejected", obj.GetName(), name)
	}

	ns := namespace
	if res.Namespaced && obj.GetNamespace() != "" {
		ns = obj.GetNamespace()
	}
	_, err = c.resourceClient(res, ns).Update(ctx, obj, metav1.UpdateOptions{})
	return err
}

// Scale sets spec.replicas via a JSON merge patch on the resource. This works
// for the built-in workloads that expose spec.replicas (deployments,
// statefulsets, replicasets, replicationcontrollers).
func (c *Client) Scale(ctx context.Context, res ResourceInfo, namespace, name string, replicas int) error {
	patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
	_, err := c.resourceClient(res, namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

// Namespaces lists namespace names, sorted.
func (c *Client) Namespaces(ctx context.Context) ([]string, error) {
	list, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for i := range list.Items {
		names = append(names, list.Items[i].Name)
	}
	sort.Strings(names)
	return names, nil
}
