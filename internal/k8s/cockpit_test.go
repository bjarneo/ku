package k8s

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestDeploymentReadyUsesDesiredReplicas(t *testing.T) {
	replicas := func(n int32) *int32 { return &n }
	tests := []struct {
		name    string
		desired *int32
		ready   int32
		want    bool
	}{
		{name: "stale zero status with desired replicas", desired: replicas(3), ready: 0, want: false},
		{name: "fully ready", desired: replicas(3), ready: 3, want: true},
		{name: "partially ready", desired: replicas(3), ready: 2, want: false},
		{name: "scaled to zero", desired: replicas(0), ready: 0, want: true},
		{name: "nil desired defaults to one", desired: nil, ready: 1, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deploymentReady(&appsv1.Deployment{
				Spec:   appsv1.DeploymentSpec{Replicas: tt.desired},
				Status: appsv1.DeploymentStatus{ReadyReplicas: tt.ready},
			})
			if got != tt.want {
				t.Fatalf("deploymentReady() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestPodRequests(t *testing.T) {
	req := func(cpu, mem string) corev1.ResourceRequirements {
		return corev1.ResourceRequirements{Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpu),
			corev1.ResourceMemory: resource.MustParse(mem),
		}}
	}
	tests := []struct {
		name    string
		pod     corev1.Pod
		wantCPU int64
		wantMem int64
	}{
		{name: "no requests", pod: corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{}}}}},
		{
			name: "app containers are summed",
			pod: corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{
				{Resources: req("250m", "128Mi")}, {Resources: req("750m", "384Mi")},
			}}},
			wantCPU: 1000, wantMem: 512 << 20,
		},
		{
			name: "largest init container wins over the app sum",
			pod: corev1.Pod{Spec: corev1.PodSpec{
				Containers:     []corev1.Container{{Resources: req("100m", "64Mi")}},
				InitContainers: []corev1.Container{{Resources: req("2", "32Mi")}, {Resources: req("50m", "1Gi")}},
			}},
			wantCPU: 2000, wantMem: 1 << 30,
		},
		{
			name: "overhead is added",
			pod: corev1.Pod{Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Resources: req("100m", "64Mi")}},
				Overhead:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m"), corev1.ResourceMemory: resource.MustParse("16Mi")},
			}},
			wantCPU: 110, wantMem: 80 << 20,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpu, mem := podRequests(&tt.pod)
			if cpu != tt.wantCPU || mem != tt.wantMem {
				t.Fatalf("podRequests() = (%d, %d), want (%d, %d)", cpu, mem, tt.wantCPU, tt.wantMem)
			}
		})
	}
}
