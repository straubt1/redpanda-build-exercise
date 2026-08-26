package skipllm

import (
	"path"
	"strings"
)

// Rule returns a category when the fetched file list is enough to skip the model.
// First match in Default wins. Empty lists never match.
type Rule func(files []string) (category string, ok bool)

// Default is the ordered skip-LLM list. Append later rules here, not in the worker loop.
func Default() []Rule {
	return []Rule{
		allMarkdown,
		allLockfiles,
	}
}

func Match(rules []Rule, files []string) (category string, ok bool) {
	for _, rule := range rules {
		if cat, hit := rule(files); hit {
			return cat, true
		}
	}
	return "", false
}

func Rationale(category string) string {
	switch category {
	case "docs":
		return "all changed files are markdown"
	case "dependency-bump":
		return "all changed files are lockfiles"
	default:
		return ""
	}
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
