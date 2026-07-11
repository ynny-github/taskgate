package cliparse

import (
	"fmt"
	"strings"
)

// UsageLine renders a single-line usage synopsis.
func (s *Spec) UsageLine(invocation string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Usage: %s", invocation)
	if len(s.Flags) > 0 {
		b.WriteString(" [flags]")
	}
	for _, a := range s.Args {
		switch {
		case a.Variadic:
			fmt.Fprintf(&b, " [%s...]", a.Name)
		case a.Required:
			fmt.Fprintf(&b, " <%s>", a.Name)
		default:
			fmt.Fprintf(&b, " [%s]", a.Name)
		}
	}
	return b.String()
}

// Help renders the full --help text: summary, usage line, argument and flag
// tables, then the body.
func (s *Spec) Help(invocation, summary, body string) string {
	var b strings.Builder
	if summary != "" {
		b.WriteString(summary)
		b.WriteString("\n\n")
	}
	b.WriteString(s.UsageLine(invocation))
	b.WriteString("\n")

	if len(s.Args) > 0 {
		b.WriteString("\nArguments:\n")
		for _, a := range s.Args {
			label := "<" + a.Name + ">"
			if a.Variadic {
				label = "[" + a.Name + "...]"
			} else if !a.Required {
				label = "[" + a.Name + "]"
			}
			fmt.Fprintf(&b, "  %-14s %s%s\n", label, a.Help, annotate(a.Choices, a.Default))
		}
	}

	b.WriteString("\nFlags:\n")
	for _, f := range s.Flags {
		head := "    " + f.Name
		if f.Short != "" {
			head = f.Short + ", " + f.Name
		}
		fmt.Fprintf(&b, "  %-14s %s%s\n", head, f.Help, annotate(f.Choices, f.Default))
	}
	fmt.Fprintf(&b, "  %-14s %s\n", "-h, --help", "Show this help")

	if body != "" {
		b.WriteString("\n")
		b.WriteString(body)
		b.WriteString("\n")
	}
	return b.String()
}

func annotate(choices []string, def *string) string {
	var parts []string
	if len(choices) > 0 {
		parts = append(parts, "choices: "+strings.Join(choices, ", "))
	}
	if def != nil {
		parts = append(parts, "default: "+*def)
	}
	if len(parts) == 0 {
		return ""
	}
	return "  (" + strings.Join(parts, "; ") + ")"
}
