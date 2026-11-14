package source

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
	assets := []ghAsset{{Name: "Q-classic.zip", URL: "c"}, {Name: "Q.zip", URL: "q"}, {Name: "Q-nolib.zip", URL: "n"}, {Name: "release.json", URL: url}}
	if a, err := pickGitHubAsset(assets); err != nil || a.Name != "Q.zip" {
		t.Fatalf("with release.json: %v %v", a, err)
	}
	if a, err := pickGitHubAsset([]ghAsset{{Name: "X-retail.zip", URL: ""}, {Name: "X-forever.zip", URL: ""}}); err != nil || a.Name != "X-forever.zip" {
		t.Fatalf("by name: %v %v", a, err)
	}
	if _, err := pickGitHubAsset([]ghAsset{{Name: "X-retail.zip", URL: ""}, {Name: "X-classic.zip", URL: ""}}); err == nil {
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

// stub points every endpoint at one test server and restores them afterwards.
func stub(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	u := serve(t, h)
	oldCurse, oldWidget, oldGH := curseAPI, cfwidgetAPI, githubAPI
	curseAPI, cfwidgetAPI, githubAPI = u, u, u
	t.Cleanup(func() { curseAPI, cfwidgetAPI, githubAPI = oldCurse, oldWidget, oldGH })
}

func TestCurseLatestPaginates(t *testing.T) {
	for _, c := range []struct {
		name      string
		total     int
		wantPages int
		wantID    string
		wantErr   bool
	}{
		{"found on the last page", 120, 3, "7", false},
		{"stops at totalCount", 50, 1, "", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			pages := 0
			stub(t, func(w http.ResponseWriter, r *http.Request) {
				pages++
				var data []curseFile
				if r.URL.Query().Get("pageIndex") == "2" {
					data = []curseFile{{ID: 7, DisplayName: "1.60.1", ReleaseType: 1, GameVersionTypeIDs: []int{foreverTypeID}}}
				} else {
					data = []curseFile{{ID: 1, DisplayName: "retail", ReleaseType: 1, GameVersionTypeIDs: []int{517}}}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "pagination": map[string]int{"totalCount": c.total}})
			})
			rel, err := curseLatest("42")
			if (err != nil) != c.wantErr || rel.ID != c.wantID || pages != c.wantPages {
				t.Fatalf("rel %+v, err %v, %d pages; want id %q, err %v, %d pages", rel, err, pages, c.wantID, c.wantErr, c.wantPages)
			}
		})
	}
}

func TestGithubLatestUsesReleaseJSON(t *testing.T) {
	var base string
	stub(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v1.2", "assets": []map[string]string{
				{"name": "r-classic.zip", "browser_download_url": base + "/dl/classic"},
				{"name": "r-nolib.zip", "browser_download_url": base + "/dl/nolib"},
				{"name": "r.zip", "browser_download_url": base + "/dl/forever", "digest": "sha256:ABCDEF0123"},
				{"name": "release.json", "browser_download_url": base + "/release.json"},
			}})
		case "/release.json":
			_, _ = w.Write([]byte(`{"releases":[
				{"filename":"r-classic.zip","metadata":[{"flavor":"classic"}]},
				{"filename":"r-nolib.zip","nolib":true,"metadata":[{"flavor":"forever"}]},
				{"filename":"r.zip","metadata":[{"flavor":"forever"}]}]}`))
		default:
			http.NotFound(w, r)
		}
	})
	base = githubAPI
	rel, err := githubLatest("o/r")
	if err != nil || rel.ID != "v1.2" || rel.URL != base+"/dl/forever" || rel.SHA256 != "abcdef0123" {
		t.Fatalf("got %+v, %v", rel, err)
	}
}

func TestGithubLatestWithoutReleases(t *testing.T) {
	stub(t, http.NotFound)
	if _, err := githubLatest("o/r"); err == nil || !strings.Contains(err.Error(), "no published releases") {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveRoutesEachKindOfRef(t *testing.T) {
	stub(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/Questie/Questie":
			_, _ = w.Write([]byte(`{"full_name":"Questie/Questie","name":"Questie"}`))
		case strings.HasPrefix(r.URL.Path, "/mods/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":       []curseFile{{ID: 1, DisplayName: "1.60.1", ReleaseType: 1, GameVersionTypeIDs: []int{foreverTypeID}}},
				"pagination": map[string]int{"totalCount": 1},
			})
		case r.URL.Path == "/wow/addons/details":
			_, _ = w.Write([]byte(`{"id":61284,"title":"Details"}`))
		case r.URL.Path == "/61284":
			_, _ = w.Write([]byte(`{"id":61284,"title":"Details"}`))
		default:
			http.NotFound(w, r)
		}
	})
	for _, c := range []struct{ ref, want string }{
		{"github:Questie/Questie", "github:Questie/Questie"},
		{"https://github.com/Questie/Questie.git", "github:Questie/Questie"},
		{"curse:61284", "curse:61284"},
		{"61284", "curse:61284"},
		{"details", "curse:61284"},
		{"https://www.curseforge.com/wow/addons/details", "curse:61284"},
	} {
		p, err := Resolve(c.ref)
		if err != nil || p.Ref() != c.want {
			t.Errorf("Resolve(%q) = %+v, %v; want %s", c.ref, p, err, c.want)
		}
	}
	if p, _ := Resolve("61284"); p.Title != "Details" {
		t.Errorf("title = %q", p.Title)
	}
}

func TestDownloadRefusesOversizeFiles(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("0123456789")) })
	old := maxDownload
	t.Cleanup(func() { maxDownload = old })

	maxDownload = 10
	dst := t.TempDir() + "/ok.zip"
	if err := Download(url, dst); err != nil {
		t.Fatalf("file at the limit: %v", err)
	}
	maxDownload = 9
	if err := Download(url, t.TempDir()+"/big.zip"); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("oversize file: err = %v", err)
	}
}

func fastRetries(t *testing.T) {
	t.Helper()
	oldDelays, oldMax := retryDelays, maxRetryAfter
	retryDelays, maxRetryAfter = []time.Duration{0, 0}, 0
	t.Cleanup(func() { retryDelays, maxRetryAfter = oldDelays, oldMax })
}

func TestGetRetriesTransientErrors(t *testing.T) {
	fastRetries(t)
	for _, code := range []int{429, 502, 503, 504} {
		calls := 0
		u := serve(t, func(w http.ResponseWriter, r *http.Request) {
			if calls++; calls == 1 {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(code)
				return
			}
			_, _ = w.Write([]byte(`{}`))
		})
		var v map[string]any
		if err := getJSON(u, &v); err != nil || calls != 2 {
			t.Errorf("HTTP %d: err %v after %d calls, want success on the 2nd", code, err, calls)
		}
	}
}

func TestGetGivesUpAfterTwoRetries(t *testing.T) {
	fastRetries(t)
	calls := 0
	u := serve(t, func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(http.StatusServiceUnavailable) })
	var v map[string]any
	if err := getJSON(u, &v); !isStatus(err, 503) || calls != 3 {
		t.Fatalf("err %v after %d calls, want 503 after 3", err, calls)
	}
}

func TestGetDoesNotRetryDefinitiveAnswers(t *testing.T) {
	fastRetries(t)
	for _, code := range []int{400, 403, 404, 500} {
		calls := 0
		u := serve(t, func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(code) })
		var v map[string]any
		if err := getJSON(u, &v); !isStatus(err, code) || calls != 1 {
			t.Errorf("HTTP %d: err %v after %d calls, want 1 call", code, err, calls)
		}
	}
}

func TestRetryDelayHonorsRetryAfterWithCap(t *testing.T) {
	err := &httpError{Code: 429, RetryAfter: time.Hour}
	if d, ok := retryDelay(err, 0); !ok || d != maxRetryAfter {
		t.Fatalf("delay %v, %v; want the cap %v", d, ok, maxRetryAfter)
	}
	if d, ok := retryDelay(&httpError{Code: 503}, 1); !ok || d != retryDelays[1] {
		t.Fatalf("delay %v, %v; want %v", d, ok, retryDelays[1])
	}
}

func TestDefinitiveAnswersAreErrNotFound(t *testing.T) {
	stub(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/mods/") && r.URL.Query().Get("pageIndex") == "0" && strings.Contains(r.URL.Path, "/200/") {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []curseFile{}, "pagination": map[string]int{"totalCount": 0}})
			return
		}
		http.NotFound(w, r)
	})
	for name, err := range map[string]error{
		"unknown slug":     func() error { _, err := CurseBySlug("nope"); return err }(),
		"unknown project":  func() error { _, err := curseLatest("100"); return err }(),
		"no forever files": func() error { _, err := curseLatest("200"); return err }(),
		"unknown repo":     func() error { _, err := githubProject("o/r"); return err }(),
	} {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: %v is not ErrNotFound", name, err)
		}
	}
}

func TestFailuresAreNotErrNotFound(t *testing.T) {
	fastRetries(t)
	stub(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) })
	if _, err := curseLatest("1"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("a 502 gave %v", err)
	}
	stub(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusAccepted) })
	if _, err := CurseBySlug("indexing"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("cfwidget indexing gave %v", err)
	}
}

func TestCurseLatestIsFetchedOncePerProject(t *testing.T) {
	requests := 0
	stub(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":       []curseFile{{ID: 3, DisplayName: "1.60.1", ReleaseType: 1, GameVersionTypeIDs: []int{foreverTypeID}}},
			"pagination": map[string]int{"totalCount": 1},
		})
	})
	for range 3 {
		if rel, err := curseLatest("42"); err != nil || rel.ID != "3" {
			t.Fatalf("got %+v, %v", rel, err)
		}
	}
	if _, err := curseByID("42"); err != nil {
		t.Fatal(err)
	}
	if requests != 2 { // one file list, plus curseByID's cfwidget title lookup
		t.Fatalf("%d requests, want 2", requests)
	}
}

func TestCurseLatestFailuresAreNotCached(t *testing.T) {
	fastRetries(t)
	fail := true
	stub(t, func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":       []curseFile{{ID: 3, DisplayName: "1.60.1", ReleaseType: 1, GameVersionTypeIDs: []int{foreverTypeID}}},
			"pagination": map[string]int{"totalCount": 1},
		})
	})
	if _, err := curseLatest("43"); err == nil {
		t.Fatal("want error")
	}
	fail = false
	if rel, err := curseLatest("43"); err != nil || rel.ID != "3" {
		t.Fatalf("after recovery: %+v, %v", rel, err)
	}
}

func TestGithubDigestOnlySha256(t *testing.T) {
	for digest, want := range map[string]string{"sha256:AB12": "ab12", "": "", "sha512:ab12": "", "ab12": ""} {
		stub(t, func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v1", "assets": []map[string]string{
				{"name": "only.zip", "browser_download_url": "u", "digest": digest},
			}})
		})
		if rel, err := githubLatest("o/r"); err != nil || rel.SHA256 != want {
			t.Errorf("digest %q: got %q, %v; want %q", digest, rel.SHA256, err, want)
		}
	}
}

func TestWagoForeverSlugs(t *testing.T) {
	old := wagoAPI
	wagoAPI = serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/data/slugs" || r.URL.Query().Get("game_version") != "forever" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"addons":{"details":{"download_count":7},"questie":{"download_count":9}}}`))
	})
	t.Cleanup(func() { wagoAPI = old })
	got, err := WagoForeverSlugs()
	if err != nil {
		t.Fatal(err)
	}
	downloads := map[string]int{}
	for _, s := range got {
		downloads[s.Slug] = s.Downloads
	}
	if len(downloads) != 2 || downloads["details"] != 7 || downloads["questie"] != 9 {
		t.Errorf("got %v", got)
	}

	wagoAPI = serve(t, func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	if _, err := WagoForeverSlugs(); err == nil {
		t.Error("a failing Wago endpoint should be an error")
	}
}

func TestLatestDispatchesBySource(t *testing.T) {
	stub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/o/r/releases/latest" {
			_, _ = w.Write([]byte(`{"tag_name":"v9","assets":[{"name":"r.zip","browser_download_url":"http://x/r.zip"}]}`))
			return
		}
		http.NotFound(w, r)
	})
	if rel, err := Latest(Project{Source: GitHub, ID: "o/r"}); err != nil || rel.Version != "v9" {
		t.Errorf("github: %+v, %v", rel, err)
	}
	if _, err := Latest(Project{Source: Curse, ID: "1"}); err == nil {
		t.Error("curse with no files should be an error")
	}
	if _, err := Latest(Project{Source: "nope", ID: "1"}); err == nil {
		t.Error("unknown source should be an error")
	}
}

func TestRateLimitIsRecognised(t *testing.T) {
	fastRetries(t)
	reset := time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
	u := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", fmt.Sprint(reset.Unix()))
		w.WriteHeader(http.StatusForbidden)
	})
	var v map[string]any
	err := getJSON(u, &v)
	if got, ok := RateLimitReset(err); !ok || !got.Equal(reset) || HTTPStatus(err) != 403 {
		t.Fatalf("err %v: reset %v %v, status %d", err, got, ok, HTTPStatus(err))
	}
	plain := fmt.Errorf("wrapped: %w", &httpError{Code: 503})
	if _, ok := RateLimitReset(plain); ok || HTTPStatus(plain) != 503 || HTTPStatus(errors.New("x")) != 0 {
		t.Fatalf("plain 503 misread: %v", plain)
	}
}
