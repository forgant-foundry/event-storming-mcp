package playbooks

import _ "embed"

// Playbook is a facilitation script the host LLM follows.
type Playbook struct {
	Server      string `json:"-"` // which server serves it: ServerEvents or ServerStories
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

//go:embed data/map-the-users-story.md
var mapTheUsersStoryContent string

const (
	ServerEvents  = "events"
	ServerStories = "stories"
)

var registry = []Playbook{
	{
		Server:      ServerEvents,
		Slug:        "run-event-storming",
		Title:       "Run Event Storming",
		Description: "Full three-phase facilitation: Big Picture, Process Modeling, and Software Design. Includes a propose-and-confirm mode for experts who ask for suggestions.",
		Content:     runEventStormingContent,
	},
	{
		Server:      ServerEvents,
		Slug:        "extract-ubiquitous-language",
		Title:       "Extract Ubiquitous Language",
		Description: "Focused glossary-building pass over an existing session; surfaces synonyms and establishes canonical phrasings.",
		Content:     extractUbiquitousLanguageContent,
	},
	{
		Server:      ServerEvents,
		Slug:        "emit-gherkin-specs",
		Title:       "Emit Gherkin Specs",
		Description: "Generates the feature files, glossary and context map; runs one batched phrasing pass, reviews scenarios (including @proposed and policy scenarios), lints steps against the glossary, and resolves warnings.",
		Content:     emitGherkinSpecsContent,
	},
	{
		Server:      ServerStories,
		Slug:        "map-the-users-story",
		Title:       "Map the User's Story",
		Description: "Story mapping linked to the event storm: frame the opportunity, model the roles, map steps in narrative flow, explore screens and what goes wrong, slice releases, and check the link to the storm.",
		Content:     mapTheUsersStoryContent,
	},
}

// ForServer returns the playbooks one server serves, without their content.
func ForServer(server string) []Playbook {
	var out []Playbook
	for _, p := range All() {
		if p.Server == server {
			out = append(out, p)
		}
	}
	return out
}

// GetForServer returns a playbook by slug if the given server serves it.
func GetForServer(server, slug string) (Playbook, bool) {
	p, ok := Get(slug)
	return p, ok && p.Server == server
}

// All returns the list of available playbooks without their content.
func All() []Playbook {
	out := make([]Playbook, len(registry))
	for i, p := range registry {
		out[i] = Playbook{Server: p.Server, Slug: p.Slug, Title: p.Title, Description: p.Description}
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
