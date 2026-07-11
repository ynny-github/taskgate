// Package validate implements `taskgate validate` / `taskgate ai validate`:
// authoring-time checks over task files under .taskgate/ (execute bit,
// shebang, annotation format, name collisions).
package validate

// Rule names — a shared contract between the engine and both renderers.
const (
	RuleExecBit      = "exec-bit"
	RuleShebang      = "shebang"
	RuleAnnotation   = "annotation"
	RuleCollision    = "collision"
	RuleDepUnknown   = "dep-unknown"
	RuleDepNotExec   = "dep-not-exec"
	RuleDepMalformed = "dep-malformed"
	RuleDepCycle     = "dep-cycle"
)

// Finding is one authoring problem. File-level findings carry Path + Message;
// collision findings carry Name + Paths. The unexported logical field is the
// entry's logical name (relative to its bucket), used only to filter by a
// name argument; it is never serialized.
type Finding struct {
	Rule    string   `json:"rule"`
	Path    string   `json:"path,omitempty"`
	Message string   `json:"message,omitempty"`
	Name    string   `json:"name,omitempty"`
	Paths   []string `json:"paths,omitempty"`

	logical string
}
