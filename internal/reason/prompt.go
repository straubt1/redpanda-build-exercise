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
	writeEvidence(&b, in, false)
	return b.String()
}

func classifyPrompt(in Input, sum Summary) string {
	var b strings.Builder
	b.WriteString(strings.TrimRight(classifyInstructions, "\n"))
	b.WriteString("\n\n")
	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("affected_area: %s\nsummary: %s\n\n", sum.AffectedArea, sum.Summary))
	writeEvidence(&b, in, true)
	return b.String()
}

func writeEvidence(b *strings.Builder, in Input, withFiles bool) {
	b.WriteString("## Input\n\n")
	b.WriteString("Title:\n")
	b.WriteString(in.Title)
	b.WriteString("\n\n")
	b.WriteString("Body:\n")
	body := in.Body
	if len(body) > maxBodyChars {
		body = body[:maxBodyChars]
	}
	b.WriteString(markdownFence(body))
	b.WriteString("\n\n")
	b.WriteString("### Changed Files:\n\n")
	additions, deletions, changes := 0, 0, 0
	for _, f := range in.Files {
		additions += f.Additions
		deletions += f.Deletions
		changes += f.Changes
	}
	fmt.Fprintf(b, "additions: %d\ndeletions: %d\nchanges: %d\n", additions, deletions, changes)
	if !withFiles {
		return
	}
	for _, f := range in.Files {
		if f.Truncated {
			fmt.Fprintf(b, "\n#### %s (%s) [truncated]\n", f.Filename, f.Status)
		} else {
			fmt.Fprintf(b, "\n#### %s (%s)\n", f.Filename, f.Status)
		}
		if f.Patch != "" {
			b.WriteString("\n")
			b.WriteString(f.Patch)
			b.WriteString("\n")
		}
	}
}

// markdownFence wraps s in a backtick run longer than any run inside s,
// so PR markdown (headings, nested fences) stays evidence, not prompt structure.
func markdownFence(s string) string {
	n := 3
	ticks := strings.Repeat("`", n)
	for strings.Contains(s, ticks) {
		n++
		ticks = strings.Repeat("`", n)
	}
	return ticks + "\n" + s + "\n" + ticks
}
