package gherkin

import (
	"strings"
	"unicode"
)

// camelWords splits PascalCase into lowercase words.
// "OrderPlaced" → ["order", "placed"]
func camelWords(s string) []string {
	var words []string
	var cur strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		isUpper := unicode.IsUpper(r)
		nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
		// start a new word on an uppercase letter, except when it's part of
		// a run of capitals that's followed by a lower (e.g. "HTTPRequest")
		if isUpper && cur.Len() > 0 && (unicode.IsLower(runes[i-1]) || nextLower) {
			words = append(words, cur.String())
			cur.Reset()
		}
		cur.WriteRune(unicode.ToLower(r))
	}
	if cur.Len() > 0 {
		words = append(words, cur.String())
	}
	return words
}

// Slug converts a name to a lowercase hyphenated filename slug.
// Handles both PascalCase ("BoundedContext" → "bounded-context") and
// space-separated names ("Ledger Management" → "ledger-management").
func Slug(s string) string {
	// Normalise spaces/underscores to hyphens first, then split CamelCase words.
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '_' || r == '-'
	})
	var parts []string
	for _, w := range words {
		parts = append(parts, camelWords(w)...)
	}
	return strings.Join(parts, "-")
}

// WhenStep formats a command name and actor as a declarative When clause.
// ("SendMessage", "User") → "the user sends a message"
func WhenStep(commandName, actorName string) string {
	words := camelWords(commandName)
	actor := "the system"
	if actorName != "" {
		actor = "the " + strings.ToLower(actorName)
	}
	if len(words) == 0 {
		return actor + " acts"
	}
	if len(words) == 1 {
		return actor + " " + conjugateVerb(words[0])
	}
	verb := conjugateVerb(words[0])
	obj := nounPhrase(words[1:])
	return actor + " " + verb + " " + obj
}

// ThenStep formats an event name as a declarative Then clause.
// "MessageSent" → "a message is sent"
// "ChannelCreated" → "a channel is created"
func ThenStep(eventName string) string {
	words := camelWords(eventName)
	if len(words) == 0 {
		return "an event occurs"
	}
	if len(words) == 1 {
		return "the " + words[0] + " event occurs"
	}
	// Last word is typically a past participle; everything before it is the noun.
	noun := nounPhrase(words[:len(words)-1])
	return noun + " is " + words[len(words)-1]
}

// ScenarioTitle produces a human-readable scenario title from a command name and actor.
// ("SendMessage", "User") → "user sends a message"
func ScenarioTitle(commandName, actorName string) string {
	words := camelWords(commandName)
	prefix := ""
	if actorName != "" {
		prefix = strings.ToLower(actorName) + " "
	}
	if len(words) == 0 {
		return prefix + "acts"
	}
	if len(words) == 1 {
		return prefix + words[0]
	}
	return prefix + conjugateVerb(words[0]) + " " + nounPhrase(words[1:])
}

func nounPhrase(words []string) string {
	if len(words) == 0 {
		return ""
	}
	noun := strings.Join(words, " ")
	if isVowelStart(noun) {
		return "an " + noun
	}
	return "a " + noun
}

// conjugateVerb adds third-person singular -s/-es to an infinitive verb.
func conjugateVerb(v string) string {
	switch {
	case strings.HasSuffix(v, "ch"), strings.HasSuffix(v, "sh"),
		strings.HasSuffix(v, "ss"), strings.HasSuffix(v, "x"),
		strings.HasSuffix(v, "zz"):
		return v + "es"
	case strings.HasSuffix(v, "y") && len(v) > 1 && !isVowelStart(string(v[len(v)-2])):
		return v[:len(v)-1] + "ies"
	default:
		return v + "s"
	}
}

func isVowelStart(s string) bool {
	if s == "" {
		return false
	}
	return strings.ContainsRune("aeiou", unicode.ToLower([]rune(s)[0]))
}
