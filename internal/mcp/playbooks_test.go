package mcp

import (
	"regexp"
	"testing"

	"github.com/forgant-foundry/event-storming-mcp/internal/playbooks"
)

// toolRef matches what a playbook writes when it means a tool: a call like
// `add_event(` or a backticked name starting with a tool verb.
var toolRef = regexp.MustCompile("\\b([a-z]+(?:_[a-z]+)+)\\(|`((?:add|update|remove|link|set|resolve|deprecate|define|lint|cluster|propose|emit|export|import|list|get|start|resume|assign)_[a-z_]+)`")

// TestPlaybooksReferenceRealTools keeps the facilitation scripts from
// pointing the LLM at tools that don't exist.
func TestPlaybooksReferenceRealTools(t *testing.T) {
	tools := New().ListTools()
	for _, meta := range playbooks.All() {
		p, _ := playbooks.Get(meta.Slug)
		for _, m := range toolRef.FindAllStringSubmatch(p.Content, -1) {
			name := m[1] + m[2]
			if _, ok := tools[name]; !ok {
				t.Errorf("playbook %s references unknown tool %q", meta.Slug, name)
			}
		}
	}
}
