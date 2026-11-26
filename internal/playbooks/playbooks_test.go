package playbooks_test

import (
	"strings"
	"testing"

	"github.com/forgant-foundry/forgant-discovery-mcp/internal/playbooks"
)

func TestAll(t *testing.T) {
	all := playbooks.All()
	if len(all) != 4 {
		t.Fatalf("expected 4 playbooks, got %d", len(all))
	}
	if n := len(playbooks.ForServer(playbooks.ServerEvents)); n != 3 {
		t.Errorf("events server: expected 3 playbooks, got %d", n)
	}
	if n := len(playbooks.ForServer(playbooks.ServerStories)); n != 1 {
		t.Errorf("stories server: expected 1 playbook, got %d", n)
	}
	if _, ok := playbooks.GetForServer(playbooks.ServerStories, "run-event-storming"); ok {
		t.Error("the stories server shouldn't serve an events playbook")
	}
	for _, p := range all {
		if p.Slug == "" || p.Title == "" || p.Description == "" {
			t.Errorf("playbook missing fields: %+v", p)
		}
		if p.Content != "" {
			t.Errorf("All() should strip content, got non-empty content for %s", p.Slug)
		}
	}
}

func TestGet(t *testing.T) {
	slugs := []string{"run-event-storming", "extract-ubiquitous-language", "emit-gherkin-specs", "map-the-users-story"}
	for _, slug := range slugs {
		p, ok := playbooks.Get(slug)
		if !ok {
			t.Errorf("Get(%q) returned false", slug)
			continue
		}
		if !strings.Contains(p.Content, "#") {
			t.Errorf("playbook %q content looks empty", slug)
		}
	}

	_, ok := playbooks.Get("nonexistent")
	if ok {
		t.Error("Get(nonexistent) should return false")
	}
}
