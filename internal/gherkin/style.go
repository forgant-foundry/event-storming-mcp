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

// Slug converts a name to a lowercase hyphenated slug for file names and tags.
// Handles PascalCase ("BoundedContext" → "bounded-context"), space-separated
// names ("Ledger Management" → "ledger-management"), and drops punctuation
// ("Whenever X, then Y" → "whenever-x-then-y").
func Slug(s string) string {
	// Split on anything that isn't a letter or digit, then split CamelCase words.
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
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
	// The participle is usually the last word ("OrderPlaced"). When the name
	// ends in a prepositional phrase instead ("ScopePassedToFactory"), split at
	// the first -ed word: "a scope is passed to factory".
	// A name that starts with its participle ("MergedToMain") has no subject
	// to put before "is", so it's used as is: "merged to main".
	if isRegularParticiple(words[0]) {
		return strings.Join(words, " ")
	}
	split := len(words) - 1
	if !isParticiple(words[split]) {
		for i := 1; i < len(words)-1; i++ {
			if isParticiple(words[i]) {
				split = i
				break
			}
		}
	}
	nounWords := words[:split]
	verb := " is "
	if isPlural(nounWords[len(nounWords)-1]) {
		verb = " are "
	}
	return nounPhrase(nounWords) + verb + strings.Join(words[split:], " ")
}

// prepositions that can open a command's object ("MergeToMain" → "merges to
// main"), which then takes no article.
var prepositions = map[string]bool{
	"to": true, "from": true, "into": true, "onto": true, "on": true, "in": true,
	"at": true, "by": true, "for": true, "with": true, "without": true,
}

// irregularParticiples are common past participles that don't end in -ed.
var irregularParticiples = map[string]bool{
	"begun": true, "broken": true, "brought": true, "built": true, "bought": true,
	"caught": true, "chosen": true, "done": true, "drawn": true, "driven": true,
	"found": true, "forgotten": true, "frozen": true, "given": true, "grown": true,
	"held": true, "hidden": true, "kept": true, "known": true, "left": true,
	"lost": true, "made": true, "met": true, "paid": true, "put": true, "run": true,
	"seen": true, "sent": true, "set": true, "shown": true, "sold": true,
	"spent": true, "taken": true, "told": true, "thrown": true, "won": true,
	"written": true,
}

func isRegularParticiple(w string) bool {
	return len(w) > 3 && strings.HasSuffix(w, "ed") && !strings.HasSuffix(w, "eed")
}

func isParticiple(w string) bool {
	return isRegularParticiple(w) || irregularParticiples[w]
}

// isPlural is a deliberately narrow guess: a trailing s that isn't part of
// -ss, -us or -is ("process", "status", "analysis"). Names it gets wrong
// should set an explicit phrasing.
func isPlural(w string) bool {
	return len(w) > 2 && strings.HasSuffix(w, "s") &&
		!strings.HasSuffix(w, "ss") && !strings.HasSuffix(w, "us") && !strings.HasSuffix(w, "is")
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
	if isPlural(words[len(words)-1]) || prepositions[words[0]] {
		return noun
	}
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
