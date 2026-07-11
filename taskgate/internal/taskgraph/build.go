package taskgraph

import (
	"bytes"
	"os"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
)

// Build resolves target and every reachable before/after dependency into a
// deduplicated graph. It returns the first structural error encountered:
// *ResolveError (unknown / non-executable / stale), *MalformedDepsError, or
// *CycleError.
func Build(target string, r Resolver) (*Graph, error) {
	b := &builder{r: r, byPath: map[string]*Node{}, onStack: map[string]bool{}}
	root, err := b.node(target)
	if err != nil {
		return nil, err
	}
	return &Graph{Root: root}, nil
}

type builder struct {
	r       Resolver
	byPath  map[string]*Node
	onStack map[string]bool
}

func (b *builder) node(name string) (*Node, error) {
	path, err := b.r.Resolve(name)
	if err != nil {
		return nil, err
	}
	if n, ok := b.byPath[path]; ok {
		return n, nil // dedup: already fully built
	}
	if b.onStack[path] {
		return nil, &CycleError{Name: name, Path: path}
	}
	b.onStack[path] = true
	defer delete(b.onStack, path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	deps, diag, err := annotation.ParseDeps(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if diag != nil {
		return nil, &MalformedDepsError{Name: name, Path: path, Reason: diag.Reason}
	}

	n := &Node{Name: name, Path: path}
	for _, dep := range deps.Before {
		child, err := b.node(dep)
		if err != nil {
			return nil, err
		}
		n.Before = append(n.Before, child)
	}
	for _, dep := range deps.After {
		child, err := b.node(dep)
		if err != nil {
			return nil, err
		}
		n.After = append(n.After, child)
	}
	b.byPath[path] = n
	return n, nil
}
