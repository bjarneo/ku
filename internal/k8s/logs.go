package k8s

import (
	"context"
	"fmt"
	"io"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LogTarget identifies one pod container log stream.
type LogTarget struct {
	Namespace string
	Pod       string
	Container string
}

// PodContainers returns the container names of a pod (init containers first,
// then regular containers), used to pick which log stream to follow.
func (c *Client) PodContainers(ctx context.Context, namespace, pod string) ([]string, error) {
	p, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return podContainerNames(p), nil
}

// DeploymentLogTargets returns every pod/container selected by a Deployment.
func (c *Client) DeploymentLogTargets(ctx context.Context, namespace, name string) ([]LogTarget, error) {
	dep, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if dep.Spec.Selector == nil {
		return nil, fmt.Errorf("deployment %q has no selector", name)
	}
	selector, err := metav1.LabelSelectorAsSelector(dep.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("deployment selector: %w", err)
	}
	if selector.Empty() {
		return nil, fmt.Errorf("deployment %q has empty selector", name)
	}

	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	sort.Slice(pods.Items, func(i, j int) bool { return pods.Items[i].Name < pods.Items[j].Name })

	var targets []LogTarget
	for i := range pods.Items {
		pod := &pods.Items[i]
		for _, container := range podContainerNames(pod) {
			targets = append(targets, LogTarget{Namespace: pod.Namespace, Pod: pod.Name, Container: container})
		}
	}
	return targets, nil
}

func podContainerNames(p *corev1.Pod) []string {
	names := make([]string, 0, len(p.Spec.InitContainers)+len(p.Spec.Containers))
	for i := range p.Spec.InitContainers {
		names = append(names, p.Spec.InitContainers[i].Name)
	}
	for i := range p.Spec.Containers {
		names = append(names, p.Spec.Containers[i].Name)
	}
	return names
}

// LogStream opens a log stream for a pod container. When follow is true the
// caller must close the returned reader (and/or cancel ctx) to stop it.
func (c *Client) LogStream(ctx context.Context, namespace, pod, container string, tail int64, follow bool) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{
		Container: container,
		Follow:    follow,
	}
	if tail >= 0 {
		opts.TailLines = &tail
	}
	return c.clientset.CoreV1().Pods(namespace).GetLogs(pod, opts).Stream(ctx)
}
