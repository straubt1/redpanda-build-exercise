package reason

import _ "embed"

// Instruction prefixes for Summarize → Classification. Evidence (files, body, title) is appended in prompt.go.
// See prompts/README.md.

//go:embed prompts/summarize.txt
var summarizeInstructions string

//go:embed prompts/classify.txt
var classifyInstructions string

//go:embed prompts/classify_repair.txt
var classifyRepairInstructions string
