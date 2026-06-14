package k8s

import (
	"context"
	"slices"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDeploymentLogTargets(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
		},
	}
	podA := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-a", Namespace: "default", Labels: map[string]string{"app": "api"}},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init"}},
			Containers:     []corev1.Container{{Name: "web"}, {Name: "sidecar"}},
		},
	}
	podB := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-b", Namespace: "default", Labels: map[string]string{"app": "api"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "worker"}}},
	}
	unmatched := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "default", Labels: map[string]string{"app": "other"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "web"}}},
	}
	c := &Client{clientset: fake.NewSimpleClientset(dep, unmatched, podB, podA)}

	got, err := c.DeploymentLogTargets(context.Background(), "default", "api")
	if err != nil {
		t.Fatalf("DeploymentLogTargets() error = %v", err)
	}
	ids := make([]string, 0, len(got))
	for _, t := range got {
		ids = append(ids, t.Pod+"/"+t.Container)
	}
	want := []string{"api-a/init", "api-a/web", "api-a/sidecar", "api-b/worker"}
	if !slices.Equal(ids, want) {
		t.Fatalf("DeploymentLogTargets() = %v, want %v", ids, want)
	}
}
