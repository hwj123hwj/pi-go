package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hwj123hwj/pi-go/internal/profile"
)

// ── JSON Response Types ───────────────────────────────────────────────────

type profileFactJSON struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Source      string    `json:"source"`
	Updated     time.Time `json:"updated"`
	AccessCount int       `json:"access_count"`
}

type profileCategoryJSON struct {
	Name  string            `json:"name"`
	Label string            `json:"label"`
	Count int               `json:"count"`
	Facts []profileFactJSON `json:"facts"`
}

type profileResponse struct {
	Categories []profileCategoryJSON `json:"categories"`
	Summary    string                `json:"summary"`
	TotalFacts int                   `json:"total_facts"`
}

// categoryLabels maps internal category names to human-readable labels.
var categoryLabels = map[string]string{
	profile.CategoryCoding:  "开发",
	profile.CategoryMusic:   "音乐",
	profile.CategoryGeneral: "通用",
}

// ── Route Registration ────────────────────────────────────────────────────

// registerProfileRoutes adds user profile endpoints to the given mux.
func (s *Server) registerProfileRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /profile", s.profileGet)
	mux.HandleFunc("DELETE /profile", s.profileDelete)
}

// ── Handlers ──────────────────────────────────────────────────────────────

func (s *Server) profileGet(w http.ResponseWriter, r *http.Request) {
	ps := s.app.Profile()
	if ps == nil {
		writeJSON(w, profileResponse{
			Categories: []profileCategoryJSON{},
			Summary:    "",
			TotalFacts: 0,
		})
		return
	}

	allFacts := ps.AllFacts()
	var categories []profileCategoryJSON
	total := 0

	// Use a deterministic order: coding, music, general, then any others
	order := []string{profile.CategoryCoding, profile.CategoryMusic, profile.CategoryGeneral}
	seen := make(map[string]bool)
	for _, cat := range order {
		seen[cat] = true
		if facts, ok := allFacts[cat]; ok {
			categories = append(categories, buildCategoryJSON(cat, facts))
			total += len(facts)
		}
	}
	for cat, facts := range allFacts {
		if !seen[cat] {
			categories = append(categories, buildCategoryJSON(cat, facts))
			total += len(facts)
		}
	}

	writeJSON(w, profileResponse{
		Categories: categories,
		Summary:    ps.Summary(),
		TotalFacts: total,
	})
}

func (s *Server) profileDelete(w http.ResponseWriter, r *http.Request) {
	ps := s.app.Profile()
	if ps == nil {
		writeError(w, http.StatusServiceUnavailable, "profile store not configured")
		return
	}

	var body struct {
		Category string `json:"category"`
		Key      string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Category == "" || body.Key == "" {
		writeError(w, http.StatusBadRequest, "category and key are required")
		return
	}

	ps.Remove(body.Category, body.Key)
	writeJSON(w, map[string]string{"status": "ok"})
}

// ── Helpers ───────────────────────────────────────────────────────────────

func buildCategoryJSON(cat string, facts []profile.Fact) profileCategoryJSON {
	label := categoryLabels[cat]
	if label == "" {
		label = cat
	}

	jsonFacts := make([]profileFactJSON, len(facts))
	for i, f := range facts {
		jsonFacts[i] = profileFactJSON{
			Key:         f.Key,
			Value:       f.Value,
			Source:      f.Source,
			Updated:     f.Updated,
			AccessCount: f.AccessCount,
		}
	}

	return profileCategoryJSON{
		Name:  cat,
		Label: label,
		Count: len(facts),
		Facts: jsonFacts,
	}
}
