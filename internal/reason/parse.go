package reason

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Labels the classifier may persist. unknown is valid model output, not only a fallback.
var Categories = map[string]struct{}{
	"security":        {},
	"feature":         {},
	"refactor":        {},
	"docs":            {},
	"dependency-bump": {},
	"unknown":         {},
}

type Classification struct {
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Rationale  string  `json:"rationale"`
}

type Summary struct {
	AffectedArea string `json:"affected_area"`
	Summary      string `json:"summary"`
}

// FirstObject returns the first complete JSON object in s, using brace matching
// and skipping braces inside strings. Trailing prose after that object is ignored.
func FirstObject(s string) (string, error) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", fmt.Errorf("no JSON object")
	}
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], nil
			}
			if depth < 0 {
				return "", fmt.Errorf("unbalanced JSON object")
			}
		}
	}
	return "", fmt.Errorf("unclosed JSON object")
}

// NormalizeCategory trims, lowercases, hyphenates spaces, and requires a known label.
func NormalizeCategory(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), " ")
	s = strings.ReplaceAll(s, " ", "-")
	if _, ok := Categories[s]; !ok {
		return "", fmt.Errorf("invalid category %q", s)
	}
	return s, nil
}

func ParseClassification(raw string) (Classification, error) {
	obj, err := FirstObject(raw)
	if err != nil {
		return Classification{}, err
	}
	var c Classification
	if err := json.Unmarshal([]byte(obj), &c); err != nil {
		return Classification{}, fmt.Errorf("unmarshal classification: %w", err)
	}
	cat, err := NormalizeCategory(c.Category)
	if err != nil {
		return Classification{}, err
	}
	c.Category = cat
	return c, nil
}

func ParseSummary(raw string) (Summary, error) {
	obj, err := FirstObject(raw)
	if err != nil {
		return Summary{}, err
	}
	var e Summary
	if err := json.Unmarshal([]byte(obj), &e); err != nil {
		return Summary{}, fmt.Errorf("unmarshal summary: %w", err)
	}
	return e, nil
}
