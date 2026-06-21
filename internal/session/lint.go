package session

import (
	"fmt"
	"strings"
	"unicode"
)

// LintIssue describes a single glossary conformance problem in a step string.
type LintIssue struct {
	Type       string `json:"type"`              // "alias" | "cross_context"
	TermID     string `json:"term_id"`
	Term       string `json:"term"`
	Found      string `json:"found"`
	UseInstead string `json:"use_instead,omitempty"`
	Message    string `json:"message"`
}

// LintStep checks stepText against the session glossary and returns any issues.
//
// It flags two things:
//   - "alias": an alias appears in the step instead of the canonical phrasing
//   - "cross_context": a term defined in a different bounded context appears in the step
//
// contextID is the bounded context the step belongs to; pass "" to skip cross-context checks.
func LintStep(s *Session, contextID, stepText string) []LintIssue {
	tokens := tokenize(stepText)
	var issues []LintIssue

	for _, term := range s.Glossary {
		canonical := strings.ToLower(term.CanonicalPhrasing)
		if canonical == "" {
			canonical = strings.ToLower(term.Term)
		}
		canonicalTokens := tokenize(canonical)

		for _, alias := range term.Aliases {
			aliasTokens := tokenize(alias)
			if containsPhrase(tokens, aliasTokens) && !containsPhrase(tokens, canonicalTokens) {
				issues = append(issues, LintIssue{
					Type:       "alias",
					TermID:     term.ID,
					Term:       term.Term,
					Found:      alias,
					UseInstead: term.CanonicalPhrasing,
					Message:    fmt.Sprintf("use %q instead of alias %q", term.CanonicalPhrasing, alias),
				})
			}
		}

		if contextID != "" && term.ContextID != "" && term.ContextID != contextID {
			termTokens := tokenize(term.Term)
			if containsPhrase(tokens, termTokens) {
				issues = append(issues, LintIssue{
					Type:    "cross_context",
					TermID:  term.ID,
					Term:    term.Term,
					Found:   term.Term,
					Message: fmt.Sprintf("term %q is defined in a different bounded context (%s)", term.Term, term.ContextID),
				})
			}
		}
	}

	return issues
}

// tokenize splits text into lowercase word tokens, stripping punctuation.
func tokenize(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// containsPhrase reports whether needle (a token slice) appears as a contiguous
// subsequence inside haystack.
func containsPhrase(haystack, needle []string) bool {
	if len(needle) == 0 {
		return false
	}
outer:
	for i := 0; i <= len(haystack)-len(needle); i++ {
		for j, w := range needle {
			if haystack[i+j] != w {
				continue outer
			}
		}
		return true
	}
	return false
}
