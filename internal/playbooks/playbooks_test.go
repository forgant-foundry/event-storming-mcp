package playbooks_test

import (
	"strings"
	"testing"

	"github.com/forgant-foundry/event-storming-mcp/internal/playbooks"
)

func TestAll(t *testing.T) {
	all := playbooks.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 playbooks, got %d", len(all))
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
	slugs := []string{"run-event-storming", "extract-ubiquitous-language", "emit-gherkin-specs"}
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
