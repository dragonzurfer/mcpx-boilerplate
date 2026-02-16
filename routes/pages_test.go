package routes

import (
	"os"
	"testing"
)

func TestLoadTemplatesIncludesAdminPages(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(".."); err != nil {
		t.Fatalf("failed to chdir to repo root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	templates, err := LoadTemplates()
	if err != nil {
		t.Fatalf("expected templates to load, got error: %v", err)
	}
	expected := []string{
		"home",
		"post",
		"pricing",
		"courses",
		"course",
		"practice",
		"account",
		"admin_dashboard",
		"admin_posts",
		"admin_courses",
		"admin_problems",
		"admin_funnel",
		"admin_promos",
		"admin_users",
		"admin_settings",
		"admin_analytics",
	}
	for _, name := range expected {
		if templates[name] == nil {
			t.Fatalf("expected template %s", name)
		}
	}
}
