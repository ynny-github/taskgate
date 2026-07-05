// Package annotation parses YAML front-matter annotation blocks from the head
// of a file. Recognizes the bare `---` envelope optionally prefixed by a
// language line-comment marker (`#`, `//`, `--`, `;`). Bare YAML (no prefix)
// is also accepted, which is the form a description file (`_index`) typically
// uses.
package annotation

import (
	"bufio"
	"bytes"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// AnnotationBlock is the summary + optional body extracted from a file.
// A zero value represents "no annotation found" (FR-009).
type AnnotationBlock struct {
	Summary string
	Body    string
}

// Diagnostic describes why a present annotation envelope is invalid. A nil
// *Diagnostic means either no envelope was present or the envelope parsed
// cleanly.
type Diagnostic struct {
	Reason string
}

type annotationDoc struct {
	Summary string `yaml:"summary"`
	Body    string `yaml:"body"`
}

// SupportedPrefixes is the ordered list of line-comment prefixes the parser
// tries when detecting the envelope delimiter. The empty string at the end
// covers bare-YAML `_index` files. Order matters only as a tie-breaker; the
// matching prefix wins on first hit.
var SupportedPrefixes = []string{"#", "//", "--", ";", ""}

// Parse scans the head of r for a YAML front-matter envelope and returns the
// decoded summary/body. On any failure (no envelope, no closer, malformed
// YAML), returns a zero AnnotationBlock and a nil error — annotations are
// best-effort per FR-009.
func Parse(r io.Reader) (AnnotationBlock, error) {
	block, _, err := parseCore(r)
	return block, err
}

// ParseStrict is like Parse but reports a *Diagnostic when an envelope is
// present yet broken (unterminated, malformed YAML, or a multi-line summary),
// so callers such as `taskgate validate` can distinguish an absent annotation
// (nil diagnostic) from a broken one.
func ParseStrict(r io.Reader) (AnnotationBlock, *Diagnostic, error) {
	return parseCore(r)
}

// parseCore performs the scan + decode once. It returns the decoded block
// (populated whenever the envelope's YAML decodes) plus an optional
// Diagnostic describing why a present envelope is invalid.
func parseCore(r io.Reader) (AnnotationBlock, *Diagnostic, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	lines := make([]string, 0, 32)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return AnnotationBlock{}, nil, err
	}

	start := 0
	if start < len(lines) && strings.HasPrefix(lines[start], "#!") {
		start++
	}

	openIdx, prefix := findOpener(lines, start)
	if openIdx < 0 {
		return AnnotationBlock{}, nil, nil
	}
	closeIdx := findCloser(lines, openIdx+1, prefix)
	if closeIdx < 0 {
		return AnnotationBlock{}, &Diagnostic{Reason: "unterminated annotation envelope"}, nil
	}

	var buf bytes.Buffer
	for _, line := range lines[openIdx+1 : closeIdx] {
		buf.WriteString(stripPrefix(line, prefix))
		buf.WriteByte('\n')
	}

	var doc annotationDoc
	if err := yaml.Unmarshal(buf.Bytes(), &doc); err != nil {
		return AnnotationBlock{}, &Diagnostic{Reason: "malformed YAML in annotation: " + err.Error()}, nil
	}

	block := AnnotationBlock{
		Summary: strings.TrimRight(doc.Summary, " \t\r\n"),
		Body:    strings.TrimRight(doc.Body, " \t\r\n"),
	}
	if strings.Contains(block.Summary, "\n") {
		return block, &Diagnostic{Reason: "summary must be a single line"}, nil
	}
	return block, nil, nil
}

func findOpener(lines []string, start int) (int, string) {
	for i := start; i < len(lines); i++ {
		for _, p := range SupportedPrefixes {
			if matchesDelimiter(lines[i], p) {
				return i, p
			}
		}
	}
	return -1, ""
}

func findCloser(lines []string, start int, prefix string) int {
	for i := start; i < len(lines); i++ {
		if matchesDelimiter(lines[i], prefix) {
			return i
		}
	}
	return -1
}

// matchesDelimiter reports whether line, after stripping prefix (with at most
// one trailing space) and trimming trailing whitespace, equals exactly "---".
func matchesDelimiter(line, prefix string) bool {
	rest, ok := stripDelimiterPrefix(line, prefix)
	if !ok {
		return false
	}
	rest = strings.TrimRight(rest, " \t\r")
	return rest == "---"
}

// stripDelimiterPrefix returns (line-without-prefix, ok). For an empty prefix
// ok is always true.
func stripDelimiterPrefix(line, prefix string) (string, bool) {
	if prefix == "" {
		return line, true
	}
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	rest := line[len(prefix):]
	if strings.HasPrefix(rest, " ") {
		rest = rest[1:]
	}
	return rest, true
}

// stripPrefix peels prefix (with up to one trailing space) from a body line.
// Preserves any further leading whitespace, which YAML literal-block scalars
// rely on for indentation.
func stripPrefix(line, prefix string) string {
	if prefix == "" {
		return line
	}
	if !strings.HasPrefix(line, prefix) {
		return line
	}
	rest := line[len(prefix):]
	if strings.HasPrefix(rest, " ") {
		rest = rest[1:]
	}
	return rest
}
