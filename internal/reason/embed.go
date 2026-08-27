package reason

import _ "embed"

// Instruction prefixes for extract → classify. Evidence (files, body, title) is appended in loop.go.
// See prompts/README.md.

//go:embed prompts/extract.txt
var extractInstructions string

//go:embed prompts/classify.txt
var classifyInstructions string

//go:embed prompts/classify_repair.txt
var classifyRepairInstructions string
