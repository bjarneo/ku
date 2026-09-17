package k8s

import (
	"errors"
	"fmt"
	"strings"
)

// Impersonation is the identity to act as for the whole session, mirroring
// kubectl's --as, --as-group and --as-uid. The zero value means "use the
// identity the kubeconfig context resolves to".
type Impersonation struct {
	// User is the username to act as, e.g. "system:admin".
	User string
	// Groups are the groups to act as. kubectl allows repeating --as-group, so
	// this is a list.
	Groups []string
	// UID is the uid to act as. Rarely needed; the API server matches it against
	// the impersonated user.
	UID string
}

// Active reports whether any impersonation was requested.
func (i Impersonation) Active() bool {
	return i.User != "" || len(i.Groups) > 0 || i.UID != ""
}

// Validate rejects groups or a uid without a user. clientcmd refuses to load a
// kubeconfig with that combination anyway ("requesting uid, groups or user-extra
// ... without impersonating a user"); catching it here reports the flag the user
// actually typed instead of a config error from deeper in the stack.
func (i Impersonation) Validate() error {
	if i.User != "" {
		return nil
	}
	switch {
	case len(i.Groups) > 0:
		return errors.New("--as-group requires --as")
	case i.UID != "":
		return errors.New("--as-uid requires --as")
	}
	return nil
}

// String renders the identity for the connectivity check and error messages.
// It returns "" when no impersonation is active.
func (i Impersonation) String() string {
	if !i.Active() {
		return ""
	}
	s := i.User
	if i.UID != "" {
		s += fmt.Sprintf(" (uid %s)", i.UID)
	}
	if len(i.Groups) > 0 {
		s += " (groups: " + strings.Join(i.Groups, ", ") + ")"
	}
	return s
}
