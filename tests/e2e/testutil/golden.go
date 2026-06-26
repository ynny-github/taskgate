package testutil

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/onsi/gomega/types"
)

// UpdateGolden is the -update flag. When true, MatchGolden writes actual
// output to the golden file on disk instead of comparing.
//
// Usage: go test ./tests/e2e/... -update
var UpdateGolden = flag.Bool("update", false, "rewrite golden files from actual output instead of comparing")

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
// the named golden file. With -update, the file is (re)written from
// actual instead of being compared (the matcher then always succeeds).
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
	p := goldenPath(m.name)
	if UpdateGolden != nil && *UpdateGolden {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return false, err
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			return false, err
		}
		m.expected = s
		return true, nil
	}
	data, err := os.ReadFile(p)
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
