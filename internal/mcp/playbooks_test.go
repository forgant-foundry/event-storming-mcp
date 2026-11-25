package mcp

import (
	"regexp"
	"testing"

	"github.com/mark3labs/mcp-go/server"

	"github.com/forgant-foundry/forgant-discovery/internal/playbooks"
)

// toolRef matches what a playbook writes when it means a tool: a call like
// `add_event(` or a backticked name starting with a tool verb.
var toolRef = regexp.MustCompile("\\b([a-z]+(?:_[a-z]+)+)\\(|`((?:add|update|remove|link|set|resolve|reopen|deprecate|define|lint|cluster|propose|emit|export|import|list|get|start|resume|assign|merge|frame|move|check)_[a-z_]+)`")

// TestPlaybooksReferenceRealTools keeps the facilitation scripts from
// pointing the LLM at tools that don't exist on the server that serves them.
func TestPlaybooksReferenceRealTools(t *testing.T) {
	servers := map[string]map[string]*server.ServerTool{
		playbooks.ServerEvents:  New().ListTools(),
		playbooks.ServerStories: NewStories().ListTools(),
	}
	for _, meta := range playbooks.All() {
		tools := servers[meta.Server]
		p, _ := playbooks.Get(meta.Slug)
		for _, m := range toolRef.FindAllStringSubmatch(p.Content, -1) {
			name := m[1] + m[2]
			if _, ok := tools[name]; !ok {
				t.Errorf("playbook %s references unknown tool %q", meta.Slug, name)
			}
		}
	}
}
