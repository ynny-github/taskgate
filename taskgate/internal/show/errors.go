package show

import (
	"errors"
	"fmt"
)

// Exit codes returned by Run. Stable contract per contracts/cli.md.
const (
	ExitSuccess          = 0
	ExitGeneric          = 1
	ExitInvalidInput     = 2
	ExitNotFound         = 3
	ExitCollision        = 4
	ExitWorkspaceMissing = 5
)

// ExitError is the sentinel cmd-layer returns to cobra so main.go can
// translate it into a single os.Exit call. Keeps os.Exit confined to main.go
// and lets cmd-level tests assert on the code via errors.As.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit %d", e.Code)
}

// CollisionReport describes a name that exists in both buckets at the same
// merged-view level.
type CollisionReport struct {
	Name  string
	Paths []string
}

// NotFoundReport describes a name that resolved to nothing.
type NotFoundReport struct {
	Name     string
	Searched []string
}

// InvalidInputReport describes input that failed validation before
// resolution was attempted.
type InvalidInputReport struct {
	Input  string
	Reason string
}

// ErrWorkspaceMissing is the sentinel returned by WorkspaceDir when the
// caller's cwd is not inside a project with `.taskgate/`.
var ErrWorkspaceMissing = errors.New(".taskgate/ not found")
