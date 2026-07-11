package taskgraph

// Execute runs the graph. For each node: run all before-deps (recursively),
// then the node's body, then — only if the body succeeded — all after-deps
// (recursively). Nodes are deduplicated by pointer, so a shared node's body
// runs at most once. Only the root node receives rootArgs. Returns the exit
// code of the first task that fails in execution order, or 0 if none fails.
func Execute(g *Graph, rootArgs []string, run Runner) int {
	e := &executor{run: run, done: map[*Node]int{}}
	e.visit(g.Root, rootArgs, true)
	return e.firstFail
}

type executor struct {
	run       Runner
	done      map[*Node]int // node -> exit code (present iff visited)
	firstFail int
}

func (e *executor) visit(node *Node, rootArgs []string, isRoot bool) int {
	if code, ok := e.done[node]; ok {
		return code // dedup
	}
	for _, dep := range node.Before {
		if code := e.visit(dep, nil, false); code != 0 {
			e.done[node] = code
			return code // skip body + after; short-circuit remaining before
		}
	}
	var args []string
	if isRoot {
		args = rootArgs
	}
	code, err := e.run(node.Path, args)
	if err != nil && code == 0 {
		code = 1 // spawn failure counts as failure
	}
	if code != 0 {
		e.done[node] = code
		if e.firstFail == 0 {
			e.firstFail = code
		}
		return code // skip after
	}
	e.done[node] = 0
	for _, dep := range node.After {
		e.visit(dep, nil, false) // an after-dep failure is recorded via firstFail
	}
	return 0
}
