package source

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serve(t *testing.T, h http.HandlerFunc) string {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestCurseLatestFiltersForever(t *testing.T) {
	files := []curseFile{
		{ID: 5, DisplayName: "12.1.17", ReleaseType: 1, GameVersions: []string{"12.1.5"}, GameVersionTypeIDs: []int{517}},
		{ID: 4, DisplayName: "1.60.32-beta", ReleaseType: 2, GameVersionTypeIDs: []int{foreverTypeID}},
		{ID: 3, DisplayName: "1.60.31-forever", ReleaseType: 1, GameVersions: []string{"1.60.1"}},
	}
	old := curseAPI
	curseAPI = serve(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": files, "pagination": map[string]int{"totalCount": len(files)}})
	})
	t.Cleanup(func() { curseAPI = old })

	rel, err := curseLatest("42")
	if err != nil || rel.ID != "3" || rel.Version != "1.60.31-forever" || !strings.HasSuffix(rel.URL, "/mods/42/files/3/download") {
		t.Fatalf("got %+v, %v", rel, err)
	}
}

func TestCurseLatestFallsBackToBeta(t *testing.T) {
	old := curseAPI
	curseAPI = serve(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":       []curseFile{{ID: 4, DisplayName: "2.0-beta", ReleaseType: 2, GameVersionTypeIDs: []int{foreverTypeID}}},
			"pagination": map[string]int{"totalCount": 1},
		})
	})
	t.Cleanup(func() { curseAPI = old })
	if rel, err := curseLatest("42"); err != nil || rel.ID != "4" {
		t.Fatalf("got %+v, %v", rel, err)
	}
}

func TestPickGitHubAsset(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"releases":[
			{"filename":"Q-nolib.zip","nolib":true,"metadata":[{"flavor":"forever"}]},
			{"filename":"Q-classic.zip","metadata":[{"flavor":"classic"}]},
			{"filename":"Q.zip","metadata":[{"flavor":"mists"},{"flavor":"forever"}]}]}`))
	})
	assets := []ghAsset{{"Q-classic.zip", "c"}, {"Q.zip", "q"}, {"Q-nolib.zip", "n"}, {"release.json", url}}
	if a, err := pickGitHubAsset(assets); err != nil || a.Name != "Q.zip" {
		t.Fatalf("with release.json: %v %v", a, err)
	}
	if a, err := pickGitHubAsset([]ghAsset{{"X-retail.zip", ""}, {"X-forever.zip", ""}}); err != nil || a.Name != "X-forever.zip" {
		t.Fatalf("by name: %v %v", a, err)
	}
	if _, err := pickGitHubAsset([]ghAsset{{"X-retail.zip", ""}, {"X-classic.zip", ""}}); err == nil {
		t.Fatal("ambiguous assets: want error")
	}
}

func TestResolveRejectsUnknownRefs(t *testing.T) {
	for _, ref := range []string{"", "some thing", "https://example.com/x/y", "curse:abc"} {
		if _, err := Resolve(ref); err == nil {
			t.Errorf("Resolve(%q): want error", ref)
		}
	}
}

func TestCurseSlug(t *testing.T) {
	if s, ok := CurseSlug("https://www.curseforge.com/wow/addons/auctionator"); !ok || s != "auctionator" {
		t.Fatalf("CurseSlug = %q, %v", s, ok)
	}
	if _, ok := CurseSlug("https://foreverartisan.app"); ok {
		t.Fatal("non-CurseForge URL matched")
	}
}
