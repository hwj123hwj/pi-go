package feishu

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSaveRoutes(t *testing.T) {
	dir := t.TempDir()
	routesFile := filepath.Join(dir, "routes.json")

	h := &Handler{
		routes:     make(map[string]*ChatRoute),
		routesFile: routesFile,
	}

	// Save some routes
	h.setRoute("oc_chat1", &ChatRoute{
		SessionID:   "sess_1",
		ProjectRoot: "/tmp/project-a",
		ChatName:    "Project A",
	})
	h.setRoute("ou_user1", &ChatRoute{
		SessionID: "sess_2",
	})

	// Verify file was written
	if _, err := os.Stat(routesFile); err != nil {
		t.Fatalf("routes file not created: %v", err)
	}

	// Create a new handler and load routes
	h2 := &Handler{
		routes:     make(map[string]*ChatRoute),
		routesFile: routesFile,
	}
	h2.loadRoutes()

	r1 := h2.getRoute("oc_chat1")
	if r1 == nil {
		t.Fatal("expected route for oc_chat1")
	}
	if r1.SessionID != "sess_1" {
		t.Errorf("sessionID = %q, want %q", r1.SessionID, "sess_1")
	}
	if r1.ProjectRoot != "/tmp/project-a" {
		t.Errorf("projectRoot = %q, want %q", r1.ProjectRoot, "/tmp/project-a")
	}
	if r1.ChatName != "Project A" {
		t.Errorf("chatName = %q, want %q", r1.ChatName, "Project A")
	}

	r2 := h2.getRoute("ou_user1")
	if r2 == nil || r2.SessionID != "sess_2" {
		t.Error("expected route for ou_user1")
	}
}

func TestWorkspaceFor(t *testing.T) {
	h := &Handler{
		routes:     make(map[string]*ChatRoute),
		routesFile: filepath.Join(t.TempDir(), "routes.json"),
		workspace:  "/default/workspace",
	}

	// No route → default workspace
	if got := h.workspaceFor("oc_unknown"); got != "/default/workspace" {
		t.Errorf("expected default workspace, got %q", got)
	}

	// Route with project root
	h.setRoute("oc_project", &ChatRoute{ProjectRoot: "/my/project"})
	if got := h.workspaceFor("oc_project"); got != "/my/project" {
		t.Errorf("expected /my/project, got %q", got)
	}

	// Route without project root → default
	h.setRoute("oc_nodir", &ChatRoute{SessionID: "sess_1"})
	if got := h.workspaceFor("oc_nodir"); got != "/default/workspace" {
		t.Errorf("expected default workspace for empty projectRoot, got %q", got)
	}
}

func TestLoadRoutes_MissingFile(t *testing.T) {
	h := &Handler{
		routes:     make(map[string]*ChatRoute),
		routesFile: filepath.Join(t.TempDir(), "nonexistent.json"),
	}
	h.loadRoutes() // should not panic
	if len(h.routes) != 0 {
		t.Errorf("expected empty routes, got %d", len(h.routes))
	}
}

func TestLoadRoutes_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	routesFile := filepath.Join(dir, "routes.json")
	os.WriteFile(routesFile, []byte("not json"), 0o644)

	h := &Handler{
		routes:     make(map[string]*ChatRoute),
		routesFile: routesFile,
	}
	h.loadRoutes() // should not panic, just warn
	if len(h.routes) != 0 {
		t.Errorf("expected empty routes on invalid JSON, got %d", len(h.routes))
	}
}
