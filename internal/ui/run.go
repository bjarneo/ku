package ui

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/bjarneo/ku/internal/k8s"
)

// Options configures a ku session.
type Options struct {
	Context    string
	Namespace  string
	Resource   string
	Theme      string
	Kubeconfig string // explicit kubeconfig path ("" = default lookup)
	Version    string // running version, for the background update check ("dev" = skip)
	Dev        bool   // developer scope: hide cluster admin resources and node ops
	Edit       bool   // start in edit mode; the session is read-only by default
	// Impersonate is the identity to act as, from --as/--as-group/--as-uid. The
	// zero value uses the kubeconfig identity. It is never persisted.
	Impersonate k8s.Impersonation
}

// Run starts the interactive TUI. The cluster connection and config load run in
// the background behind a splash screen (see startupCmd / adoptStartup); flags
// take precedence over the remembered context/namespace from the last session.
func Run(opts Options) error {
	saved, hasSaved := loadState()
	ctxName := opts.Context
	if ctxName == "" && hasSaved {
		ctxName = saved.Context
	}
	if err := k8s.ValidateKubeconfig(ctxName, opts.Kubeconfig, opts.Impersonate); err != nil {
		if opts.Context != "" || ctxName == "" {
			return err
		}
		if err := k8s.ValidateKubeconfig("", opts.Kubeconfig, opts.Impersonate); err != nil {
			return err
		}
	}

	// Theme precedence: --theme flag, then $KU_THEME, then the remembered choice.
	name := opts.Theme
	if name == "" {
		name = os.Getenv("KU_THEME")
	}
	if name == "" {
		name = saved.Theme
	}
	th := PickTheme(name)

	app := App{theme: th, keys: defaultKeys(), splash: true, opts: opts, saved: saved, hasSaved: hasSaved}
	app.dev = opts.Dev
	app.impersonate = opts.Impersonate
	// Safe by default: the session starts read-only unless --edit is passed. It
	// can be toggled at runtime from the command palette.
	app.readOnly = !opts.Edit
	app.spin = newSpinner(th)

	m, err := tea.NewProgram(app).Run()
	if err != nil {
		return err
	}
	// A fatal connection error is reported here rather than from a goroutine.
	if fin, ok := m.(App); ok && fin.startErr != nil {
		return fin.startErr
	}
	// A clean farewell on the normal screen once the alt-screen is torn down.
	fmt.Printf("\n  %s\n\n", goodbye(th))
	return nil
}

// goodbye is the farewell line printed after a clean quit.
func goodbye(th Theme) string {
	return th.HeaderVal.Render("ku") + th.Dim.Render(" · see you next time · "+creatorHandle)
}
