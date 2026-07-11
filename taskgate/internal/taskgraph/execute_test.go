package taskgraph

import (
	"strings"
	"testing"
)

// recorder builds a Runner that appends each executed path's basename to a log
// and returns a preset exit code per name.
func recorder(log *[]string, fail map[string]int) Runner {
	return func(path string, args []string) (int, error) {
		name := path[strings.LastIndex(path, "/")+1:]
		*log = append(*log, name)
		if code, ok := fail[name]; ok {
			return code, nil
		}
		return 0, nil
	}
}

func n(name string) *Node { return &Node{Name: name, Path: "/t/" + name} }

func TestExecute_ImmediateAfterOrder(t *testing.T) {
	// deploy(before=[build], after=[notify]); build(after=[clean])
	build := n("build")
	build.After = []*Node{n("clean")}
	deploy := n("deploy")
	deploy.Before = []*Node{build}
	deploy.After = []*Node{n("notify")}

	var log []string
	code := Execute(&Graph{Root: deploy}, nil, recorder(&log, nil))
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	got := strings.Join(log, ",")
	if got != "build,clean,deploy,notify" {
		t.Fatalf("order = %q, want build,clean,deploy,notify", got)
	}
}

func TestExecute_DedupRunsOnce(t *testing.T) {
	d := n("d")
	b := n("b")
	b.Before = []*Node{d}
	c := n("c")
	c.Before = []*Node{d}
	a := n("a")
	a.Before = []*Node{b, c}

	var log []string
	Execute(&Graph{Root: a}, nil, recorder(&log, nil))
	count := 0
	for _, x := range log {
		if x == "d" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("d ran %d times, want 1 (log=%v)", count, log)
	}
}

func TestExecute_BeforeFailSkipsBodyAndAfter(t *testing.T) {
	deploy := n("deploy")
	deploy.Before = []*Node{n("build")}
	deploy.After = []*Node{n("notify")}

	var log []string
	code := Execute(&Graph{Root: deploy}, nil, recorder(&log, map[string]int{"build": 3}))
	if code != 3 {
		t.Fatalf("exit = %d, want 3", code)
	}
	if strings.Contains(strings.Join(log, ","), "deploy") || strings.Contains(strings.Join(log, ","), "notify") {
		t.Fatalf("deploy/notify must not run; log=%v", log)
	}
}

func TestExecute_BodyFailSkipsOwnAfter(t *testing.T) {
	deploy := n("deploy")
	deploy.After = []*Node{n("notify")}

	var log []string
	code := Execute(&Graph{Root: deploy}, nil, recorder(&log, map[string]int{"deploy": 1}))
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if strings.Contains(strings.Join(log, ","), "notify") {
		t.Fatalf("notify must be skipped; log=%v", log)
	}
}

func TestExecute_AfterFailureSetsExitCode(t *testing.T) {
	deploy := n("deploy")
	deploy.After = []*Node{n("notify")}
	var log []string
	code := Execute(&Graph{Root: deploy}, nil, recorder(&log, map[string]int{"notify": 4}))
	if code != 4 {
		t.Fatalf("exit = %d, want 4 (after-dep failure must set exit code)", code)
	}
	// deploy body ran (success) and notify ran (and failed)
	joined := strings.Join(log, ",")
	if !strings.Contains(joined, "deploy") || !strings.Contains(joined, "notify") {
		t.Fatalf("both deploy and notify should have run; log=%v", log)
	}
}

func TestExecute_RootReceivesArgs(t *testing.T) {
	root := n("deploy")
	root.Before = []*Node{n("build")}
	var gotArgs []string
	run := func(path string, args []string) (int, error) {
		if strings.HasSuffix(path, "/deploy") {
			gotArgs = args
		} else if len(args) != 0 {
			t.Errorf("dependency %s got args %v, want none", path, args)
		}
		return 0, nil
	}
	Execute(&Graph{Root: root}, []string{"--env", "prod"}, run)
	if strings.Join(gotArgs, " ") != "--env prod" {
		t.Fatalf("root args = %v, want [--env prod]", gotArgs)
	}
}
