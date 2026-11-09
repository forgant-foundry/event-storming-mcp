package playbooks

import _ "embed"

// Playbook is a facilitation script the host LLM follows.
type Playbook struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Content     string `json:"-"`
}

//go:embed data/run-event-storming.md
var runEventStormingContent string

//go:embed data/extract-ubiquitous-language.md
var extractUbiquitousLanguageContent string

//go:embed data/emit-gherkin-specs.md
var emitGherkinSpecsContent string

var registry = []Playbook{
	{
		Slug:        "run-event-storming",
		Title:       "Run Event Storming",
		Description: "Full three-phase facilitation: Big Picture, Process Modeling, and Software Design. Includes a propose-and-confirm mode for experts who ask for suggestions.",
		Content:     runEventStormingContent,
	},
	{
		Slug:        "extract-ubiquitous-language",
		Title:       "Extract Ubiquitous Language",
		Description: "Focused glossary-building pass over an existing session; surfaces synonyms and establishes canonical phrasings.",
		Content:     extractUbiquitousLanguageContent,
	},
	{
		Slug:        "emit-gherkin-specs",
		Title:       "Emit Gherkin Specs",
		Description: "Generates the feature files, glossary and context map; runs one batched phrasing pass, reviews scenarios (including @proposed and policy scenarios), lints steps against the glossary, and resolves warnings.",
		Content:     emitGherkinSpecsContent,
	},
}

// All returns the list of available playbooks without their content.
func All() []Playbook {
	out := make([]Playbook, len(registry))
	for i, p := range registry {
		out[i] = Playbook{Slug: p.Slug, Title: p.Title, Description: p.Description}
	}
	return out
}

// Get returns a playbook by slug, including its full content.
func Get(slug string) (Playbook, bool) {
	for _, p := range registry {
		if p.Slug == slug {
			return p, true
		}
	}
	return Playbook{}, false
}
