package session

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// LintIssue describes a single glossary conformance problem in a step string.
type LintIssue struct {
	Type       string `json:"type"` // "alias" | "cross_context"
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
//   - "cross_context": a term defined in a different bounded context appears in the
//     step, unless the term is shared with the step's context
//
// A match that lies inside a longer occurrence of another term's name or
// canonical phrasing doesn't count: "cell" inside "the cell agent", or the
// alias "entry" inside another term's "the recurring entry".
//
// contextID is the bounded context the step belongs to; pass "" to skip cross-context checks.
func LintStep(s *Session, contextID, stepText string) []LintIssue {
	tokens := tokenize(stepText)
	spans := glossarySpans(s, tokens)
	var issues []LintIssue

	for _, term := range s.Glossary {
		canonical := strings.ToLower(term.CanonicalPhrasing)
		if canonical == "" {
			canonical = strings.ToLower(term.Term)
		}
		canonicalTokens := tokenize(canonical)

		for _, alias := range term.Aliases {
			if spans.standalone(term.ID, tokens, tokenize(alias)) && !containsPhrase(tokens, canonicalTokens) {
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

		if contextID != "" && term.ContextID != "" && term.ContextID != contextID &&
			!slices.Contains(term.SharedWith, contextID) {
			if spans.standalone(term.ID, tokens, tokenize(term.Term)) {
				issues = append(issues, LintIssue{
					Type:    "cross_context",
					TermID:  term.ID,
					Term:    term.Term,
					Found:   term.Term,
					Message: fmt.Sprintf("term %q is defined in a different bounded context (%s); if the contexts share it on purpose, add this context to the term's shared_with", term.Term, contextLabel(s, term.ContextID)),
				})
			}
		}
	}

	return issues
}

// span is where a glossary term's name or canonical phrasing occurs in a step.
type span struct {
	termID     string
	start, end int // token indexes, end exclusive
}

type spanSet []span

// glossarySpans finds every occurrence of every term's name and canonical
// phrasing in tokens.
func glossarySpans(s *Session, tokens []string) spanSet {
	var spans spanSet
	for _, t := range s.Glossary {
		for _, phrase := range []string{t.Term, t.CanonicalPhrasing} {
			needle := tokenize(phrase)
			for _, i := range phraseIndexes(tokens, needle) {
				spans = append(spans, span{t.ID, i, i + len(needle)})
			}
		}
	}
	return spans
}

// standalone reports whether needle occurs in tokens at least once outside
// any longer span belonging to a different term.
func (spans spanSet) standalone(termID string, tokens, needle []string) bool {
	for _, i := range phraseIndexes(tokens, needle) {
		covered := false
		for _, sp := range spans {
			if sp.termID != termID && sp.start <= i && i+len(needle) <= sp.end && sp.end-sp.start > len(needle) {
				covered = true
				break
			}
		}
		if !covered {
			return true
		}
	}
	return false
}

// tokenize splits text into lowercase word tokens, stripping punctuation.
func tokenize(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// phraseIndexes returns every index at which needle (a token slice) starts as
// a contiguous subsequence of haystack.
func phraseIndexes(haystack, needle []string) []int {
	if len(needle) == 0 {
		return nil
	}
	var out []int
outer:
	for i := 0; i <= len(haystack)-len(needle); i++ {
		for j, w := range needle {
			if haystack[i+j] != w {
				continue outer
			}
		}
		out = append(out, i)
	}
	return out
}

// containsPhrase reports whether needle (a token slice) appears as a contiguous
// subsequence inside haystack.
func containsPhrase(haystack, needle []string) bool {
	return len(phraseIndexes(haystack, needle)) > 0
}

// contextLabel names a context as "Name, id", or just the ID if it's unknown.
func contextLabel(s *Session, id string) string {
	for _, c := range s.BoundedContexts {
		if c.ID == id {
			return fmt.Sprintf("%s, %s", c.Name, id)
		}
	}
	return id
}
