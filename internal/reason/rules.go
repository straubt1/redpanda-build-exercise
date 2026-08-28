package reason

import (
	"fmt"
	"path"
	"strings"
)

// Rule returns a category when the fetched file list is enough to skip the Model.
// First match in defaultRules wins. Empty lists never match.
type Rule func(files []string) (category string, ok bool)

func defaultRules() []Rule {
	return []Rule{
		allMarkdown,
		allLockfiles,
	}
}

func match(rules []Rule, files []string) (category string, ok bool) {
	for _, rule := range rules {
		if cat, hit := rule(files); hit {
			return cat, true
		}
	}
	return "", false
}

func matchRules(in Input) (Outcome, bool) {
	names := make([]string, 0, len(in.Files))
	for _, f := range in.Files {
		names = append(names, f.Filename)
	}
	cat, ok := match(defaultRules(), names)
	if !ok {
		return Outcome{}, false
	}
	return Outcome{
		Category:  cat,
		Source:    "rule",
		Rationale: ruleRationale(cat) + "; " + enrichmentRationale(in),
	}, true
}

func ruleRationale(category string) string {
	switch category {
	case "docs":
		return "all changed files are markdown"
	case "dependency-bump":
		return "all changed files are lockfiles"
	default:
		return ""
	}
}

func enrichmentRationale(in Input) string {
	if strings.TrimSpace(in.Body) == "" && len(in.Files) == 0 {
		return "empty body and no files"
	}
	parts := make([]string, 0, len(in.Files))
	for _, f := range in.Files {
		parts = append(parts, f.Filename+" ("+f.Status+")")
	}
	return fmt.Sprintf("body_len=%d files=%d: %s", len(in.Body), len(in.Files), strings.Join(parts, ", "))
}

func allMarkdown(files []string) (string, bool) {
	if len(files) == 0 {
		return "", false
	}
	for _, f := range files {
		if !strings.HasSuffix(f, ".md") {
			return "", false
		}
	}
	return "docs", true
}

// Lockfile names from decisions.md. Match the basename GitHub returns; case-sensitive.
var lockfiles = map[string]struct{}{
	"package-lock.json":   {},
	"yarn.lock":           {},
	"pnpm-lock.yaml":      {},
	"npm-shrinkwrap.json": {},
	"Cargo.lock":          {},
	"go.sum":              {},
	"poetry.lock":         {},
	"Pipfile.lock":        {},
	"Gemfile.lock":        {},
	"composer.lock":       {},
	"bun.lock":            {},
	"bun.lockb":           {},
}

func allLockfiles(files []string) (string, bool) {
	if len(files) == 0 {
		return "", false
	}
	for _, f := range files {
		if _, ok := lockfiles[path.Base(f)]; !ok {
			return "", false
		}
	}
	return "dependency-bump", true
}
