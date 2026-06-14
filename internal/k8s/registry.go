package k8s

import (
	"slices"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceInfo describes a single API resource kind discovered on the server.
type ResourceInfo struct {
	Group      string
	Version    string
	Resource   string // plural, e.g. "pods"
	Kind       string // e.g. "Pod"
	Singular   string // e.g. "pod"
	ShortNames []string
	Namespaced bool
}

// GVR returns the GroupVersionResource used by the dynamic client.
func (r ResourceInfo) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: r.Group, Version: r.Version, Resource: r.Resource}
}

// Key is the canonical lookup key (the plural resource name, group-qualified
// when not in the core group) used to identify a resource unambiguously.
func (r ResourceInfo) Key() string {
	if r.Group == "" {
		return r.Resource
	}
	return r.Resource + "." + r.Group
}

// Title is a short human label for the resource shown in the header.
func (r ResourceInfo) Title() string {
	return r.Resource
}

// IsPod reports whether this resource is the core Pod type.
func (r ResourceInfo) IsPod() bool {
	return r.Group == "" && r.Resource == "pods"
}

// IsDeployment reports whether this resource is an apps Deployment list.
func (r ResourceInfo) IsDeployment() bool {
	return r.Resource == "deployments" && (r.Group == "" || r.Group == "apps")
}

// Scalable reports whether the resource exposes spec.replicas.
func (r ResourceInfo) Scalable() bool {
	switch r.Resource {
	case "deployments", "statefulsets", "replicasets", "replicationcontrollers":
		return true
	}
	return false
}

// Restartable reports whether the resource supports a rolling restart.
func (r ResourceInfo) Restartable() bool {
	switch r.Resource {
	case "deployments", "statefulsets", "daemonsets":
		return true
	}
	return false
}

// IsCronJob reports whether this is a batch CronJob list.
func (r ResourceInfo) IsCronJob() bool {
	return r.Group == "batch" && r.Resource == "cronjobs"
}

// IsNodes reports whether this is the core Node list.
func (r ResourceInfo) IsNodes() bool {
	return r.Group == "" && r.Resource == "nodes"
}

// Registry is an in-memory catalog of discovered resources with alias lookup.
type Registry struct {
	all   []ResourceInfo
	byKey map[string]ResourceInfo
}

// loadRegistry queries discovery for the server's preferred resources and
// builds the catalog. It tolerates partial discovery failures.
func (c *Client) loadRegistry() error {
	lists, err := c.disco.ServerPreferredResources()
	reg := &Registry{byKey: map[string]ResourceInfo{}}

	for _, list := range lists {
		if list == nil {
			continue
		}
		gv, perr := schema.ParseGroupVersion(list.GroupVersion)
		if perr != nil {
			continue
		}
		for _, ar := range list.APIResources {
			// Skip subresources like pods/log, deployments/scale.
			if strings.Contains(ar.Name, "/") {
				continue
			}
			if !canList(ar.Verbs) {
				continue
			}
			ri := ResourceInfo{
				Group:      gv.Group,
				Version:    gv.Version,
				Resource:   ar.Name,
				Kind:       ar.Kind,
				Singular:   ar.SingularName,
				ShortNames: ar.ShortNames,
				Namespaced: ar.Namespaced,
			}
			reg.add(ri)
		}
	}

	sort.Slice(reg.all, func(i, j int) bool {
		if reg.all[i].Group != reg.all[j].Group {
			return reg.all[i].Group < reg.all[j].Group
		}
		return reg.all[i].Resource < reg.all[j].Resource
	})

	c.registry = reg
	return err
}

func canList(verbs []string) bool {
	return slices.Contains(verbs, "list")
}

func (reg *Registry) add(ri ResourceInfo) {
	reg.all = append(reg.all, ri)
	// The first resource to claim a key wins. ServerPreferredResources returns
	// the preferred version first, so this keeps the canonical mapping.
	keys := []string{
		strings.ToLower(ri.Resource),
		strings.ToLower(ri.Singular),
		strings.ToLower(ri.Kind),
		ri.Key(),
	}
	for _, sn := range ri.ShortNames {
		keys = append(keys, strings.ToLower(sn))
	}
	for _, k := range keys {
		if k == "" {
			continue
		}
		if _, exists := reg.byKey[k]; !exists {
			reg.byKey[k] = ri
		}
	}
}

// Resolve maps a user query (plural, singular, kind, short name, or
// group-qualified key) to a resource. Returns false if unknown.
func (reg *Registry) Resolve(query string) (ResourceInfo, bool) {
	if reg == nil {
		return ResourceInfo{}, false
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if ri, ok := reg.byKey[q]; ok {
		return ri, true
	}
	// Allow "resource.group" even if only the plural was indexed.
	if i := strings.Index(q, "."); i > 0 {
		if ri, ok := reg.byKey[q[:i]]; ok {
			return ri, true
		}
	}
	return ResourceInfo{}, false
}

// All returns the full catalog sorted by group then resource.
func (reg *Registry) All() []ResourceInfo {
	if reg == nil {
		return nil
	}
	return reg.all
}
