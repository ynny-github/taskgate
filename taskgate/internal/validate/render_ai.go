package validate

import (
	"encoding/json"
	"io"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

// validationEnvelope is the single AI wire-format document (ADR-0003 style).
type validationEnvelope struct {
	Kind     string    `json:"kind"`
	OK       bool      `json:"ok"`
	Findings []Finding `json:"findings"`
}

type aiErrorEnvelope struct {
	Kind    string `json:"kind"`
	Err     string `json:"error"`
	Message string `json:"message"`
}

// renderAI writes one JSON validation envelope terminated by "\n".
func renderAI(stdout io.Writer, findings []Finding) (int, error) {
	if findings == nil {
		findings = []Finding{}
	}
	env := validationEnvelope{Kind: "validation", OK: len(findings) == 0, Findings: findings}
	if err := writeJSONLine(stdout, env); err != nil {
		return show.ExitGeneric, err
	}
	if len(findings) == 0 {
		return show.ExitSuccess, nil
	}
	return show.ExitGeneric, nil
}

// writeAIError emits a structured error envelope on the given writer (stdout,
// per ADR-0003, so AI clients parse one stream regardless of outcome).
func writeAIError(w io.Writer, code, message string) error {
	return writeJSONLine(w, aiErrorEnvelope{Kind: "error", Err: code, Message: message})
}

func writeJSONLine(w io.Writer, payload any) error {
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
