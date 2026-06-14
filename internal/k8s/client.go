// Package k8s wraps client-go to provide a small, generic interface the TUI
// uses to talk to any Kubernetes cluster: resource discovery, server-side
// table listing, YAML get, edit/apply, delete, scale and pod logs.
package k8s

import (
	"errors"
	"fmt"
	"sort"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client holds the connection to a single cluster/context. It is rebuilt from
// scratch when the user switches context.
type Client struct {
	// ContextName is the kubeconfig context currently in use.
	ContextName string
	// Host is the API server URL, shown in the header.
	Host string
	// Namespace is the default namespace declared by the context ("" if none).
	Namespace string
	// DiscoveryWarning is set when resource discovery was only partial (e.g. an
	// aggregated API is down), so the UI can warn that the catalog is incomplete.
	DiscoveryWarning string

	restConfig *rest.Config
	clientset  kubernetes.Interface
	dynamic    dynamic.Interface
	disco      discovery.DiscoveryInterface

	registry   *Registry
	contexts   []string
	kubeconfig string // explicit kubeconfig path, reused when switching context
}

// NewClient builds a client from the kubeconfig. kubeconfigPath, if non-empty,
// overrides the default lookup ($KUBECONFIG, then ~/.kube/config). If
// contextOverride is non-empty it selects that context instead of the
// kubeconfig's current-context.
func NewClient(contextOverride, kubeconfigPath string) (*Client, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		rules.ExplicitPath = kubeconfigPath
	}
	overrides := &clientcmd.ConfigOverrides{}
	if contextOverride != "" {
		overrides.CurrentContext = contextOverride
	}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)

	restCfg, err := cc.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	// Be responsive: the TUI fires frequent list calls on refresh.
	restCfg.QPS = 50
	restCfg.Burst = 100
	if restCfg.UserAgent == "" {
		restCfg.UserAgent = "kli"
	}
	// API warning headers are useful in logs but corrupt the terminal UI when the
	// default client-go handler writes them directly to stderr.
	restCfg.WarningHandlerWithContext = rest.NoWarnings{}

	ns, _, err := cc.Namespace()
	if err != nil {
		ns = ""
	}

	raw, err := cc.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("read kubeconfig: %w", err)
	}
	ctxName := raw.CurrentContext
	if contextOverride != "" {
		ctxName = contextOverride
	}
	contexts := make([]string, 0, len(raw.Contexts))
	for name := range raw.Contexts {
		contexts = append(contexts, name)
	}
	sort.Strings(contexts)

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("build clientset: %w", err)
	}
	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("build dynamic client: %w", err)
	}
	disco, err := discovery.NewDiscoveryClientForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("build discovery client: %w", err)
	}

	c := &Client{
		ContextName: ctxName,
		Host:        restCfg.Host,
		Namespace:   ns,
		restConfig:  restCfg,
		clientset:   clientset,
		dynamic:     dyn,
		disco:       disco,
		contexts:    contexts,
		kubeconfig:  kubeconfigPath,
	}

	// Discovery may be partial (an aggregated API can be down) but should not
	// stop the app; we keep whatever resolved and surface a warning.
	if err := c.loadRegistry(); err != nil {
		if c.registry == nil {
			return nil, fmt.Errorf("discover resources: %w", err)
		}
		c.DiscoveryWarning = discoveryWarning(err)
	}
	return c, nil
}

// discoveryWarning summarizes a partial-discovery error for the status line.
func discoveryWarning(err error) string {
	if groups, ok := failedGroups(err); ok {
		if len(groups) == 1 {
			return "partial discovery: " + groups[0] + " unavailable"
		}
		return fmt.Sprintf("partial discovery: %d API groups unavailable", len(groups))
	}
	return "partial discovery: some resources unavailable"
}

func failedGroups(err error) ([]string, bool) {
	var gd *discovery.ErrGroupDiscoveryFailed
	if !errors.As(err, &gd) {
		return nil, false
	}
	groups := make([]string, 0, len(gd.Groups))
	for gv := range gd.Groups {
		groups = append(groups, gv.String())
	}
	sort.Strings(groups)
	return groups, true
}

// Registry exposes the discovered resource catalog.
func (c *Client) Registry() *Registry { return c.registry }

// Contexts returns the sorted list of context names in the kubeconfig.
func (c *Client) Contexts() []string { return c.contexts }

// Kubeconfig returns the explicit kubeconfig path in use ("" for the default),
// so a context switch can reuse it.
func (c *Client) Kubeconfig() string { return c.kubeconfig }
