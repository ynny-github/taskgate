package show

import (
	"encoding/json"
	"io"
	"strings"
)

// envelopes per contracts/ai-output.md.

type childRecord struct {
	Path    string  `json:"path"`
	Kind    string  `json:"kind"`
	Summary *string `json:"summary"`
}

type listingEnvelope struct {
	Kind     string        `json:"kind"`
	Audience string        `json:"audience"`
	Entries  []childRecord `json:"entries"`
}

type taskEnvelope struct {
	Kind     string  `json:"kind"`
	Path     string  `json:"path"`
	Summary  *string `json:"summary"`
	Body     string  `json:"body,omitempty"`
	Audience string  `json:"audience"`
}

type directoryEnvelope struct {
	Kind     string        `json:"kind"`
	Path     string        `json:"path"`
	Audience string        `json:"audience"`
	Entries  []childRecord `json:"entries"`
}

type errorEnvelope struct {
	Kind     string   `json:"kind"`
	Err      string   `json:"error"`
	Message  string   `json:"message"`
	Name     string   `json:"name,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	Searched []string `json:"searched,omitempty"`
	Input    string   `json:"input,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	Path     string   `json:"path,omitempty"`
}

// renderAI marshals payload as a single JSON document terminated by "\n".
func renderAI(w io.Writer, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = w.Write([]byte{'\n'})
	return err
}

func renderAIError(w io.Writer, env errorEnvelope) error {
	return renderAI(w, env)
}

// kindString maps EntryKind onto the wire-format discriminator.
func kindString(k EntryKind) string {
	if k == EntryKindDirectory {
		return "directory"
	}
	return "task"
}

// summaryPtr returns nil for an empty summary so the JSON encoder emits null.
func summaryPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// audienceString returns the wire label for an Audience.
func audienceString(a Audience) string {
	if a == AudienceAI {
		return "ai"
	}
	return "human"
}

func childRecords(entries []Entry) []childRecord {
	out := make([]childRecord, 0, len(entries))
	for _, e := range entries {
		out = append(out, childRecord{
			Path:    e.Path,
			Kind:    kindString(e.Kind),
			Summary: summaryPtr(e.Annotation.Summary),
		})
	}
	return out
}

func renderAIRootListing(w io.Writer, entries []Entry) error {
	return renderAI(w, listingEnvelope{
		Kind:     "listing",
		Audience: "ai",
		Entries:  childRecords(entries),
	})
}

// runName maps a physical entry path onto its run-style name by dropping the
// ".taskgate/<bucket>/" prefix (bucket is human, ai, or shared). Returns ""
// when path has fewer than three slash-separated segments.
func runName(path string) string {
	segs := strings.Split(path, "/")
	if len(segs) < 3 {
		return ""
	}
	return strings.Join(segs[2:], "/")
}
