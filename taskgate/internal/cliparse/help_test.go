package cliparse

import (
	"strings"
	"testing"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
)

func TestUsageLine(t *testing.T) {
	spec := compile(t, annotation.RawSpec{
		Args: []annotation.RawArg{
			{Name: "env", Required: true},
			{Name: "files", Variadic: true},
		},
		Flags: []annotation.RawFlag{{Name: "--tag"}},
	})
	got := spec.UsageLine("taskgate run deploy")
	want := "Usage: taskgate run deploy [flags] <env> [files...]"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestHelp_ContainsSections(t *testing.T) {
	spec := compile(t, annotation.RawSpec{
		Args:  []annotation.RawArg{{Name: "env", Help: "Target environment", Choices: []string{"staging", "prod"}, Required: true}},
		Flags: []annotation.RawFlag{{Name: "--dry-run", Short: "-n", Type: "bool", Help: "Skip side effects"}},
	})
	out := spec.Help("taskgate run deploy", "Deploy to an environment.", "Body line.")
	for _, frag := range []string{
		"Deploy to an environment.",
		"Usage: taskgate run deploy",
		"Arguments:",
		"<env>",
		"Target environment",
		"choices: staging, prod",
		"Flags:",
		"-n, --dry-run",
		"Skip side effects",
		"-h, --help",
		"Body line.",
	} {
		if !strings.Contains(out, frag) {
			t.Errorf("help missing %q\n---\n%s", frag, out)
		}
	}
}
