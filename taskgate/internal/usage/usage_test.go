package usage

import (
	"strings"
	"testing"
)

func TestGuide_ContainsKeyGuidance(t *testing.T) {
	g := Guide()
	for _, want := range []string{
		"taskgate ai show",
		"taskgate ai run",
		"taskgate ai validate",
		"summary",
		"body",
		"before",
		"after",
		"snapshot install",
	} {
		if !strings.Contains(g, want) {
			t.Errorf("Guide() missing %q", want)
		}
	}
	if !strings.HasSuffix(g, "\n") {
		t.Errorf("Guide() should end with a newline")
	}
}

func TestPointer_ContainsCommandAndNoMarkers(t *testing.T) {
	p := Pointer()
	if !strings.Contains(p, "taskgate ai usage") {
		t.Errorf("Pointer() should tell the agent to run 'taskgate ai usage', got: %q", p)
	}
	if strings.Contains(p, "taskgate:begin") || strings.Contains(p, "taskgate:end") {
		t.Errorf("Pointer() must not embed the markers, got: %q", p)
	}
}
