package reason

import (
	"fmt"
	"strings"
)

const maxBodyChars = 8000

func summarizePrompt(in Input) string {
	var b strings.Builder
	b.WriteString(strings.TrimRight(summarizeInstructions, "\n"))
	b.WriteString("\n\n")
	writeEvidence(&b, in)
	return b.String()
}

func classifyPrompt(in Input, sum Summary) string {
	var b strings.Builder
	b.WriteString(strings.TrimRight(classifyInstructions, "\n"))
	b.WriteString("\n\n")
	b.WriteString("Summary:\n")
	b.WriteString(fmt.Sprintf("affected_area: %s\nsummary: %s\n\n", sum.AffectedArea, sum.Summary))
	writeEvidence(&b, in)
	return b.String()
}

func writeEvidence(b *strings.Builder, in Input) {
	b.WriteString("Changed files:\n")
	if len(in.Files) == 0 {
		b.WriteString("(none)\n")
	}
	for _, f := range in.Files {
		b.WriteString(fmt.Sprintf("### %s (%s)\n", f.Filename, f.Status))
		if f.Patch != "" {
			b.WriteString(f.Patch)
			b.WriteString("\n")
		}
	}
	b.WriteString("\nPR body:\n")
	body := in.Body
	if len(body) > maxBodyChars {
		body = body[:maxBodyChars]
	}
	if strings.TrimSpace(body) == "" {
		b.WriteString("(empty)\n")
	} else {
		b.WriteString(body)
		b.WriteString("\n")
	}
	b.WriteString("\nTitle (optional context only; do not decide from this alone): ")
	if strings.TrimSpace(in.Title) == "" {
		b.WriteString("(none)\n")
		return
	}
	b.WriteString(in.Title)
	b.WriteString("\n")
}
