package ui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/bjarneo/ku/internal/k8s"
)

// plugin is a validated shortcut from the config. Scopes stay as the user's
// strings and resolve against the registry at match time, because the registry
// is rebuilt on every context switch and a CRD may exist in one cluster only.
type plugin struct {
	key        string // normalized Bubble Tea key string, e.g. "ctrl+o", "X", "b"
	binding    key.Binding
	desc       string
	scopes     []string // lowercased; "all" matches every resource
	command    string
	args       []string // raw; expanded per invocation
	background bool
	confirm    bool
}

// pluginRunMsg carries a confirmed plugin run back into Update. The confirm
// overlay only yields a tea.Cmd, but a terminal plugin must mutate the model
// to open the overlay, so the run happens on the next Update instead.
type pluginRunMsg struct {
	plugin plugin
	vars   map[string]string
}

// pluginModifierOrder is the order Bubble Tea prints modifiers in a keystroke,
// so a user-written "shift+ctrl+x" still matches what the runtime reports.
var pluginModifierOrder = []string{"ctrl", "alt", "shift", "meta", "hyper", "super"}

// normalizePluginKey turns a user-written key into the string Bubble Tea
// reports for that keystroke. Printable keys report their text, so a shifted
// letter is the uppercase letter ("X"), not "shift+x". Returns "" for an empty
// key.
func normalizePluginKey(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if raw == "+" {
		return raw
	}
	parts := strings.Split(raw, "+")
	base := parts[len(parts)-1]
	mods := map[string]bool{}
	for _, m := range parts[:len(parts)-1] {
		mods[strings.ToLower(strings.TrimSpace(m))] = true
	}
	single := len([]rune(base)) == 1
	if !single {
		// Named keys ("f5", "enter", "space") are reported lowercase.
		base = strings.ToLower(base)
	}
	if single && len(mods) == 1 && mods["shift"] {
		return strings.ToUpper(base)
	}
	if single && len(mods) > 0 {
		// Modified letters are reported by their base code, which is lowercase.
		base = strings.ToLower(base)
	}
	var sb strings.Builder
	for _, m := range pluginModifierOrder {
		if mods[m] {
			sb.WriteString(m)
			sb.WriteString("+")
		}
	}
	sb.WriteString(base)
	return sb.String()
}

// reservedKeys lists every key the table screen already handles. Plugins may
// not shadow them: a built-in that silently stops working is worse than a
// startup warning.
func reservedKeys(k keyMap) map[string]bool {
	reserved := map[string]bool{}
	for _, g := range k.groups() {
		for _, b := range g.keys {
			for _, s := range b.Keys() {
				reserved[s] = true
			}
		}
	}
	// Handled directly by string in the key dispatch rather than via keyMap.
	for _, s := range []string{"h", "left", "right", "enter", "ctrl+c", "ctrl+\\"} {
		reserved[s] = true
	}
	return reserved
}

// pluginCatalog validates the configured plugins and returns the usable ones
// plus one warning per skipped entry. Order is preserved; on a duplicate key
// the first plugin wins.
func (c Config) pluginCatalog(keys keyMap) ([]plugin, []string) {
	if len(c.Plugins) == 0 {
		return nil, nil
	}
	reserved := reservedKeys(keys)
	seen := map[string]string{} // key -> desc of the plugin that claimed it
	var out []plugin
	var warnings []string
	for i, pc := range c.Plugins {
		label := pc.Desc
		if label == "" {
			label = "plugin " + itoa(i+1)
		}
		k := normalizePluginKey(pc.Key)
		switch {
		case k == "":
			warnings = append(warnings, fmt.Sprintf("%s: missing key", label))
			continue
		case strings.TrimSpace(pc.Command) == "":
			warnings = append(warnings, fmt.Sprintf("%s: missing command", label))
			continue
		case len(pc.Scopes) == 0:
			warnings = append(warnings, fmt.Sprintf("%s: missing scopes", label))
			continue
		case reserved[k]:
			warnings = append(warnings, fmt.Sprintf("%s: key %q is reserved", label, k))
			continue
		}
		if first, dup := seen[k]; dup {
			warnings = append(warnings, fmt.Sprintf("%s: key %q already used by %s", label, k, first))
			continue
		}
		desc := strings.TrimSpace(pc.Desc)
		if desc == "" {
			desc = filepath.Base(pc.Command)
		}
		scopes := make([]string, 0, len(pc.Scopes))
		for _, s := range pc.Scopes {
			scopes = append(scopes, strings.ToLower(strings.TrimSpace(s)))
		}
		seen[k] = desc
		out = append(out, plugin{
			key:        k,
			binding:    key.NewBinding(key.WithKeys(k), key.WithHelp(k, desc)),
			desc:       desc,
			scopes:     scopes,
			command:    strings.TrimSpace(pc.Command),
			args:       pc.Args,
			background: pc.Background,
			confirm:    pc.Confirm,
		})
	}
	return out, warnings
}

// matches reports whether the plugin applies to the resource on screen. Scopes
// resolve through the registry so aliases work ("deploy", "Deployment"). A
// literal comparison covers a nil registry and CRDs the current cluster lacks.
func (p plugin) matches(res k8s.ResourceInfo, reg *k8s.Registry) bool {
	for _, s := range p.scopes {
		if s == "all" {
			return true
		}
		if ri, ok := reg.Resolve(s); ok {
			if ri.Key() == res.Key() {
				return true
			}
			continue
		}
		if s == strings.ToLower(res.Key()) || s == strings.ToLower(res.Resource) ||
			(res.Singular != "" && s == strings.ToLower(res.Singular)) ||
			(res.Kind != "" && s == strings.ToLower(res.Kind)) {
			return true
		}
		for _, sn := range res.ShortNames {
			if s == strings.ToLower(sn) {
				return true
			}
		}
	}
	return false
}

func (a App) pluginRegistry() *k8s.Registry {
	if a.client == nil {
		return nil
	}
	return a.client.Registry()
}

// activePlugins returns the plugins that apply to the current resource, in
// config order. It feeds the footer, the palette and the key dispatch.
func (a App) activePlugins() []plugin {
	if len(a.plugins) == 0 {
		return nil
	}
	reg := a.pluginRegistry()
	var out []plugin
	for _, p := range a.plugins {
		if p.matches(a.res, reg) {
			out = append(out, p)
		}
	}
	return out
}

func (a App) pluginForKey(msg tea.KeyMsg) (plugin, bool) {
	for _, p := range a.activePlugins() {
		if key.Matches(msg, p.binding) {
			return p, true
		}
	}
	return plugin{}, false
}

// pluginVars builds the substitution map for one row. NAMESPACE is empty for
// cluster-scoped resources; otherwise it falls back from the row to the view's
// namespace and then to the context default, matching what kubectl would use.
// KUBECONFIG is only set when ku was given an explicit path, so an unset value
// falls through to the real environment.
func (a App) pluginVars(row k8s.Row) map[string]string {
	vars := map[string]string{
		"NAME":      row.Name,
		"RESOURCE":  a.res.Key(),
		"NAMESPACE": "",
		"CONTEXT":   "",
		"CLUSTER":   "",
	}
	if a.client != nil {
		vars["CONTEXT"] = a.client.ContextName
		vars["CLUSTER"] = a.client.ClusterName
		if kc := a.client.Kubeconfig(); kc != "" {
			vars["KUBECONFIG"] = kc
		}
	}
	if a.res.Namespaced {
		switch {
		case row.Namespace != "":
			vars["NAMESPACE"] = row.Namespace
		case a.namespace != "":
			vars["NAMESPACE"] = a.namespace
		case a.client != nil:
			vars["NAMESPACE"] = a.client.Namespace
		}
	}
	return vars
}

// expandPluginArgs substitutes $VAR and ${VAR} in each argument. Names outside
// the plugin set come from the environment, so "$HOME/bin/tool" keeps working.
func expandPluginArgs(args []string, vars map[string]string) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = os.Expand(arg, func(name string) string {
			if v, ok := vars[name]; ok {
				return v
			}
			return os.Getenv(name)
		})
	}
	return out
}

// pluginEnv is the inherited environment plus the plugin variables, appended
// last so they win over any inherited value of the same name.
func pluginEnv(vars map[string]string) []string {
	env := os.Environ()
	for k, v := range vars {
		env = append(env, k+"="+v)
	}
	return env
}

// runPlugin runs p against the selected row, asking first when the plugin
// requests confirmation. No selection is a silent no-op like the built-ins.
func (a App) runPlugin(p plugin) (tea.Model, tea.Cmd) {
	row, ok := a.table.selected()
	if !ok {
		return a, nil
	}
	vars := a.pluginVars(row)
	if p.confirm {
		msg := pluginRunMsg{plugin: p, vars: vars}
		return a.confirmAction(p.desc, fmt.Sprintf("Run %s on %s?", p.desc, qualified(vars["NAMESPACE"], row.Name)), false,
			func() tea.Msg { return msg })
	}
	return a.execPlugin(p, vars)
}

// execPlugin launches the command: detached with captured output for
// background plugins, or inside the terminal overlay otherwise.
func (a App) execPlugin(p plugin, vars map[string]string) (tea.Model, tea.Cmd) {
	args := expandPluginArgs(p.args, vars)
	env := pluginEnv(vars)
	if p.background {
		cmd := exec.Command(p.command, args...)
		cmd.Env = env
		a.setStatus(p.desc+": running", false)
		return a, runPluginBackground(p.desc, cmd)
	}
	batch, err := a.startPTY(p.desc, p.command, args, append(env, "TERM=xterm-256color"))
	if err != nil {
		a.setStatus(p.desc+": "+trimErr(err), true)
		return a, nil
	}
	return a, batch
}

// runPluginBackground runs the command to completion off the UI thread and
// reports the outcome as a status notice. Stdin is left unattached so a
// program that waits for input fails instead of hanging forever.
func runPluginBackground(desc string, cmd *exec.Cmd) tea.Cmd {
	return func() tea.Msg {
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			text := strings.TrimSpace(stderr.String())
			if text == "" {
				text = strings.TrimSpace(stdout.String())
			}
			if text == "" {
				text = err.Error()
			}
			return statusMsg{text: truncate(desc+": "+firstLine(text), 160), err: true}
		}
		out := strings.TrimSpace(stdout.String())
		if out == "" {
			return statusMsg{text: desc + ": done"}
		}
		return statusMsg{text: truncate(desc+": "+firstLine(out), 160)}
	}
}

// pluginHelpGroups renders the plugins as one extra column in the help screen.
func pluginHelpGroups(ps []plugin) []helpGroup {
	if len(ps) == 0 {
		return nil
	}
	keys := make([]key.Binding, 0, len(ps))
	for _, p := range ps {
		keys = append(keys, p.binding)
	}
	return []helpGroup{{"Plugins", keys}}
}
