package testutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/onsi/gomega/types"
)

// Golden reads the named golden file under testdata/golden/ relative to
// the calling test package's directory (go test's working directory).
// Use Lines/Cols for short inline expectations; reach for Golden only when
// the expected output is large enough that inline becomes hard to read.
func Golden(name string) string {
	data, err := os.ReadFile(goldenPath(name))
	if err != nil {
		panic(fmt.Sprintf("read golden %q: %v", name, err))
	}
	return string(data)
}

// MatchGolden returns a Gomega matcher that compares the actual string to
// the named golden file. Mismatches always fail — the matcher never
// rewrites the golden file. To accept an intentional behavior change,
// regenerate the golden by hand (read the failing test's diff, edit the
// .golden file, rerun).
//
//	Expect(out.Stdout).To(testutil.MatchGolden("dir_with_index"))
func MatchGolden(name string) types.GomegaMatcher {
	return &goldenMatcher{name: name}
}

func goldenPath(name string) string {
	return filepath.Join("testdata/golden", name+".golden")
}

type goldenMatcher struct {
	name     string
	expected string
}

func (m *goldenMatcher) Match(actual interface{}) (bool, error) {
	s, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("MatchGolden expects a string actual, got %T", actual)
	}
	data, err := os.ReadFile(goldenPath(m.name))
	if err != nil {
		return false, fmt.Errorf("read golden %q: %w", m.name, err)
	}
	m.expected = string(data)
	return s == m.expected, nil
}

func (m *goldenMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("expected stdout to match golden %q\n\nactual:\n%s\nexpected:\n%s\n", m.name, actual, m.expected)
}

func (m *goldenMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("expected stdout NOT to match golden %q", m.name)
}
