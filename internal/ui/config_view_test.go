package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/bjarneo/ku/internal/k8s"
)

func TestRenderConfigDecodesSecretData(t *testing.T) {
	th := PickTheme("ansi")
	res := k8s.ResourceInfo{Resource: "secrets", Kind: "Secret"}
	obj := map[string]interface{}{
		"type": "Opaque",
		"data": map[string]interface{}{
			"password": "aHVudGVyMg==",
		},
	}

	out := renderConfig(th, res, obj, 80, nil)
	if !strings.Contains(out, "Decoded Data") {
		t.Fatalf("decoded data section missing from config view:\n%s", out)
	}
	if !strings.Contains(out, "hunter2") {
		t.Fatalf("decoded secret value missing from config view:\n%s", out)
	}
	if !strings.Contains(out, "7B decoded") {
		t.Fatalf("decoded secret size metadata missing from config view:\n%s", out)
	}
	if strings.Contains(out, "aHVudGVyMg==") {
		t.Fatalf("encoded secret value leaked into config view:\n%s", out)
	}
	if strings.Contains(out, "YAML") {
		t.Fatalf("config overview should not include raw yaml:\n%s", out)
	}
}

func TestRenderConfigShowsPodUsageAndIssues(t *testing.T) {
	th := PickTheme("ansi")
	res := k8s.ResourceInfo{Resource: "pods", Kind: "Pod"}
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "api",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"cpu": "100m", "memory": "128Mi"},
						"limits":   map[string]interface{}{"cpu": "500m", "memory": "512Mi"},
					},
				},
			},
		},
		"status": map[string]interface{}{
			"phase": "Running",
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "False", "reason": "ContainersNotReady"},
			},
			"containerStatuses": []interface{}{
				map[string]interface{}{
					"name":         "api",
					"restartCount": 3,
					"state": map[string]interface{}{
						"waiting": map[string]interface{}{"reason": "CrashLoopBackOff"},
					},
				},
			},
		},
	}
	usage := &k8s.PodUsage{CPUUsedMilli: 25, MemUsedBytes: 64 * 1024 * 1024}

	out := renderConfig(th, res, obj, 80, usage)
	for _, want := range []string{"Usage", "25m live", "64Mi live", "cpu 100m", "mem 128Mi", "Health", "CrashLoopBackOff", "restarts", "3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("config view missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "YAML") {
		t.Fatalf("config overview should not include raw yaml:\n%s", out)
	}
}

func TestRenderConfigShowsIngressRuleDetails(t *testing.T) {
	th := PickTheme("ansi")
	res := k8s.ResourceInfo{Resource: "ingresses", Kind: "Ingress"}
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"ingressClassName": "nginx",
			"defaultBackend": map[string]interface{}{
				"service": map[string]interface{}{
					"name": "fallback",
					"port": map[string]interface{}{"number": 8080},
				},
			},
			"tls": []interface{}{
				map[string]interface{}{
					"hosts":      []interface{}{"app.example.com"},
					"secretName": "app-cert",
				},
			},
			"rules": []interface{}{
				map[string]interface{}{
					"host": "app.example.com",
					"http": map[string]interface{}{
						"paths": []interface{}{
							map[string]interface{}{
								"path":     "/",
								"pathType": "Prefix",
								"backend": map[string]interface{}{
									"service": map[string]interface{}{
										"name": "web",
										"port": map[string]interface{}{"number": 80},
									},
								},
							},
							map[string]interface{}{
								"path":     "/api",
								"pathType": "Prefix",
								"backend": map[string]interface{}{
									"service": map[string]interface{}{
										"name": "api",
										"port": map[string]interface{}{"name": "http"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	out := renderConfig(th, res, obj, 140, nil)
	for _, want := range []string{
		"class", "nginx",
		"default", "fallback:8080",
		"tls 1", "app.example.com -> app-cert",
		"rule 1", "app.example.com", "2 paths",
		"path 1.1", "/ (Prefix) -> web:80",
		"path 1.2", "/api (Prefix) -> api:http",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("config view missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "YAML") {
		t.Fatalf("config overview should not include raw yaml:\n%s", out)
	}
}

func TestRenderConfigPutsStatusBeforeOverview(t *testing.T) {
	th := PickTheme("ansi")
	tests := []struct {
		name string
		res  k8s.ResourceInfo
		obj  map[string]interface{}
		want []string
	}{
		{
			name: "deployment",
			res:  k8s.ResourceInfo{Resource: "deployments", Kind: "Deployment"},
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{"type": "Available", "status": "False", "reason": "MinimumReplicasUnavailable"},
					},
				},
			},
			want: []string{"Available=False"},
		},
		{
			name: "job",
			res:  k8s.ResourceInfo{Resource: "jobs", Kind: "Job"},
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{"type": "Failed", "status": "True", "reason": "BackoffLimitExceeded"},
					},
				},
			},
			want: []string{"BackoffLimitExceeded"},
		},
		{
			name: "cronjob",
			res:  k8s.ResourceInfo{Resource: "cronjobs", Kind: "CronJob"},
			obj: map[string]interface{}{
				"status": map[string]interface{}{"lastScheduleTime": "2026-06-14T10:00:00Z"},
			},
			want: []string{"last run"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := renderConfig(th, tt.res, tt.obj, 80, nil)
			statusAt := strings.Index(out, "Status")
			overviewAt := strings.Index(out, "Overview")
			if statusAt < 0 || overviewAt < 0 || statusAt > overviewAt {
				t.Fatalf("status section should appear before overview:\n%s", out)
			}
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Fatalf("config view missing %q:\n%s", want, out)
				}
			}
		})
	}
}

// configView is now a pager: it must re-lay-out the width-sensitive summary on
// resize rather than blank or keep the stale width, and it supports copy.
func TestConfigViewReRendersOnResize(t *testing.T) {
	c := newConfigView(PickTheme("ansi"))
	c.setSize(80, 20)
	res := k8s.ResourceInfo{Resource: "secrets", Kind: "Secret"}
	obj := map[string]interface{}{
		"type": "Opaque",
		"data": map[string]interface{}{"password": "aHVudGVyMg=="},
	}
	c.setObject(res, "secret/db", obj, nil)
	if strings.TrimSpace(ansi.Strip(c.vp.View())) == "" {
		t.Fatal("config view is blank after setObject")
	}

	c.setSize(48, 12) // narrower: must re-render, not blank or overflow
	view := c.vp.View()
	if strings.TrimSpace(ansi.Strip(view)) == "" {
		t.Fatal("config view blanked after resize")
	}
	for _, ln := range strings.Split(view, "\n") {
		if w := ansi.StringWidth(ln); w > 48 {
			t.Fatalf("resized config line exceeds width 48 (%d): %q", w, ansi.Strip(ln))
		}
	}

	// copyAll returns the plain summary (ANSI stripped).
	if strings.Contains(c.copyAll(), "\x1b") {
		t.Fatal("config copyAll must strip ANSI")
	}
}

func TestConfigViewResizePreservesScroll(t *testing.T) {
	c := newConfigView(PickTheme("ansi"))
	c.setSize(80, 8)
	data := make(map[string]interface{}, 30)
	for i := 0; i < 30; i++ {
		data[fmt.Sprintf("key-%02d", i)] = strings.Repeat("value", 4)
	}
	obj := map[string]interface{}{
		"data": data,
	}
	c.setObject(k8s.ResourceInfo{Resource: "configmaps", Kind: "ConfigMap"}, "configmap/app", obj, nil)
	c.vp.SetYOffset(5)
	before := c.vp.YOffset()
	if before == 0 {
		t.Fatal("precondition failed: config view did not scroll")
	}

	c.setSize(72, 8)

	if got := c.vp.YOffset(); got != before {
		t.Fatalf("resize changed YOffset from %d to %d", before, got)
	}
}

func TestRenderConfigSeparatesLongSecretKeys(t *testing.T) {
	th := PickTheme("ansi")
	res := k8s.ResourceInfo{Resource: "secrets", Kind: "Secret"}
	obj := map[string]interface{}{
		"type": "Opaque",
		"data": map[string]interface{}{
			"POSTGRES_REPLICATION_PASSWORD": "cmVwbGljYXRvcnBhc3M=",
		},
	}

	out := renderConfig(th, res, obj, 72, nil)
	plain := ansi.Strip(out)
	if strings.Contains(plain, "POSTGRES_REPLICATION_PASSWORDreplicatorpass") {
		t.Fatalf("long secret key was not separated from value:\n%s", out)
	}
	if !strings.Contains(plain, "  replicatorpass") {
		t.Fatalf("decoded value missing expected separation:\n%s", out)
	}
}
