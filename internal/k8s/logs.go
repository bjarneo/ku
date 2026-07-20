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

// LogMode selects the current live container or its finite previous instance.
type LogMode int

const (
	LogCurrent LogMode = iota
	LogPrevious
)

// PodContainer describes one container available for logs or exec.
type PodContainer struct {
	Name              string
	PreviousAvailable bool
}

// PodContainers returns the containers of a pod (init containers first, then
// regular containers) and whether each has a previous terminated instance.
func (c *Client) PodContainers(ctx context.Context, namespace, pod string) ([]PodContainer, error) {
	p, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return podContainers(p), nil
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
	containers := podContainers(p)
	names := make([]string, len(containers))
	for i := range containers {
		names[i] = containers[i].Name
	}
	return names
}

func podContainers(p *corev1.Pod) []PodContainer {
	previous := make(map[string]bool, len(p.Status.InitContainerStatuses)+len(p.Status.ContainerStatuses))
	for _, status := range p.Status.InitContainerStatuses {
		previous[status.Name] = hasPreviousInstance(status)
	}
	for _, status := range p.Status.ContainerStatuses {
		previous[status.Name] = hasPreviousInstance(status)
	}

	containers := make([]PodContainer, 0, len(p.Spec.InitContainers)+len(p.Spec.Containers))
	for i := range p.Spec.InitContainers {
		name := p.Spec.InitContainers[i].Name
		containers = append(containers, PodContainer{Name: name, PreviousAvailable: previous[name]})
	}
	for i := range p.Spec.Containers {
		name := p.Spec.Containers[i].Name
		containers = append(containers, PodContainer{Name: name, PreviousAvailable: previous[name]})
	}
	return containers
}

func hasPreviousInstance(status corev1.ContainerStatus) bool {
	return status.RestartCount > 0 && status.LastTerminationState.Terminated != nil
}

// LogStream opens logs for the selected container instance. Current logs follow
// until the caller closes the reader or cancels ctx; previous logs are finite.
func (c *Client) LogStream(ctx context.Context, namespace, pod, container string, tail int64, mode LogMode) (io.ReadCloser, error) {
	opts := podLogOptions(container, tail, mode)
	return c.clientset.CoreV1().Pods(namespace).GetLogs(pod, opts).Stream(ctx)
}

func podLogOptions(container string, tail int64, mode LogMode) *corev1.PodLogOptions {
	previous := mode == LogPrevious
	opts := &corev1.PodLogOptions{
		Container: container,
		Follow:    !previous,
		Previous:  previous,
	}
	if tail >= 0 {
		opts.TailLines = &tail
	}
	return opts
}
