// Package taskgraph builds and executes a task's before/after dependency
// lifecycle. Build resolves the reachable graph and detects structural errors
// (unknown reference, non-executable, malformed deps, cycle); Execute runs the
// graph with recursive, deduplicated, immediate-after-on-success semantics.
package taskgraph

import "fmt"

// ResolveErrorKind classifies why a name failed to resolve.
type ResolveErrorKind int

const (
	// ResolveUnknown means the name matched no task in the audience view.
	ResolveUnknown ResolveErrorKind = iota
	// ResolveNotExecutable means the file exists but lacks an execute bit.
	ResolveNotExecutable
	// ResolveStale means an ai-run snapshot is out of date vs. its source.
	ResolveStale
)

// ResolveError is returned by a Resolver when a name cannot be turned into a
// runnable path.
type ResolveError struct {
	Name   string
	Kind   ResolveErrorKind
	Detail string
}

func (e *ResolveError) Error() string {
	return fmt.Sprintf("dependency %q: %s", e.Name, e.Detail)
}

// Resolver maps a run-style task name to an absolute executable path.
type Resolver interface {
	Resolve(name string) (path string, err error)
}

// Runner executes the task at path with args and returns its process exit code
// (0 = success). A non-nil error signals a spawn failure (treated as failure).
type Runner func(path string, args []string) (exitCode int, err error)

// Node is one task in the dependency graph. Nodes are deduplicated by Path, so
// a task reached through multiple edges is the same pointer.
type Node struct {
	Name   string
	Path   string
	Before []*Node
	After  []*Node
}

// Graph is the dependency graph rooted at the CLI target.
type Graph struct{ Root *Node }

// CycleError reports a dependency cycle discovered during Build.
type CycleError struct{ Name, Path string }

func (e *CycleError) Error() string {
	return fmt.Sprintf("dependency cycle detected at %q (%s)", e.Name, e.Path)
}

// MalformedDepsError reports a present-but-invalid before/after list.
type MalformedDepsError struct{ Name, Path, Reason string }

func (e *MalformedDepsError) Error() string {
	return fmt.Sprintf("task %q (%s): %s", e.Name, e.Path, e.Reason)
}
