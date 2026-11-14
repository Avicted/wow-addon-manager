// Package source finds WoW Forever builds of addons on CurseForge and GitHub.
//
// CurseForge is reached through the website's own API (no key needed): the documented API
// requires a key. Slugs are mapped to project ids through cfwidget, and Wago's keyless slug
// list feeds search.
package source

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// UserAgent identifies wam to every service; cli sets the version.
var UserAgent = "wam/dev (WoW Forever addon manager)"

const (
	// CurseForge game-version type of the WoW Forever flavor (client 1.60.x, interface 16001).
	foreverTypeID     = 88568
	foreverVersionPfx = "1.60."
	curseRelease      = 1
)

// Endpoints are variables so tests can point them at httptest servers.
var (
	curseAPI    = "https://www.curseforge.com/api/v1"
	cfwidgetAPI = "https://api.cfwidget.com"
	githubAPI   = "https://api.github.com"
	wagoAPI     = "https://addons.wago.io"

	// maxDownload is the largest release file wam accepts; tests lower it.
	maxDownload int64 = 1 << 30

	apiClient      = &http.Client{Timeout: 60 * time.Second}
	downloadClient = &http.Client{Timeout: 15 * time.Minute}
)

// Sources.
const (
	Curse  = "curse"
	GitHub = "github"
)

// Project identifies an addon at its source: a CurseForge project id or a GitHub "owner/repo".
type Project struct {
	Source, ID, Title string
}

// Ref is the form wam install accepts: "curse:123" or "github:owner/repo".
func (p Project) Ref() string { return p.Source + ":" + p.ID }

// Release is one downloadable build of a project for the Forever client.
type Release struct {
	ID, Version, URL string
}

type httpError struct {
	Code int
	URL  string
}

func (e *httpError) Error() string { return fmt.Sprintf("HTTP %d from %s", e.Code, e.URL) }

func isStatus(err error, code int) bool {
	var he *httpError
	return errors.As(err, &he) && he.Code == code
}

func get(c *http.Client, u string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" && strings.HasPrefix(u, githubAPI) {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, &httpError{resp.StatusCode, u}
	}
	return resp, nil
}

func getJSON(u string, v any) error {
	resp, err := get(apiClient, u)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("decoding %s: %w", u, err)
	}
	return nil
}

// Download saves u to dst, refusing anything over 1 GiB.
func Download(u, dst string) error {
	resp, err := get(downloadClient, u)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(resp.Body, maxDownload+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > maxDownload {
		err = fmt.Errorf("download from %s is larger than %d bytes", u, maxDownload)
	}
	return err
}

var (
	curseURL  = regexp.MustCompile(`curseforge\.com/wow/addons/([A-Za-z0-9_-]+)`)
	githubURL = regexp.MustCompile(`github\.com/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)`)
	digits    = regexp.MustCompile(`^[0-9]+$`)
)

// CurseSlug extracts the slug from a CurseForge addon URL.
func CurseSlug(s string) (string, bool) {
	m := curseURL.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// IsProjectID reports whether s is a numeric CurseForge project id.
func IsProjectID(s string) bool { return digits.MatchString(s) }

// Resolve turns what the user typed into a Project:
// "curse:123", "123", a CurseForge URL or slug, "github:owner/repo" or a GitHub URL.
func Resolve(ref string) (Project, error) {
	ref = strings.TrimSpace(ref)
	switch {
	case strings.HasPrefix(ref, "github:"):
		return githubProject(strings.TrimPrefix(ref, "github:"))
	case githubURL.MatchString(ref):
		return githubProject(githubURL.FindStringSubmatch(ref)[1])
	case strings.HasPrefix(ref, "curse:"):
		return curseByID(strings.TrimPrefix(ref, "curse:"))
	case IsProjectID(ref):
		return curseByID(ref)
	case curseURL.MatchString(ref):
		slug, _ := CurseSlug(ref)
		return CurseBySlug(slug)
	case ref != "" && !strings.ContainsAny(ref, "/: "):
		return CurseBySlug(ref)
	}
	return Project{}, fmt.Errorf("don't know how to install %q: use a CurseForge URL, slug or project id, or a GitHub repo URL", ref)
}

type cfwidget struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// CurseBySlug looks a CurseForge slug up through cfwidget.
func CurseBySlug(slug string) (Project, error) {
	var w cfwidget
	err := getJSON(cfwidgetAPI+"/wow/addons/"+url.PathEscape(strings.ToLower(slug)), &w)
	if isStatus(err, http.StatusAccepted) {
		return Project{}, fmt.Errorf("cfwidget is still indexing %q, try again in a minute", slug)
	}
	if isStatus(err, http.StatusNotFound) || err == nil && w.ID == 0 {
		return Project{}, fmt.Errorf("no CurseForge addon with slug %q", slug)
	}
	if err != nil {
		return Project{}, err
	}
	return Project{Curse, strconv.FormatInt(w.ID, 10), w.Title}, nil
}

// curseByID checks the project has Forever files; cfwidget only supplies the display title.
func curseByID(id string) (Project, error) {
	if !IsProjectID(id) {
		return Project{}, fmt.Errorf("CurseForge project id must be a number, got %q", id)
	}
	if _, err := curseLatest(id); err != nil {
		return Project{}, err
	}
	p := Project{Source: Curse, ID: id}
	var w cfwidget
	if getJSON(cfwidgetAPI+"/"+id, &w) == nil {
		p.Title = w.Title
	}
	return p, nil
}

func githubProject(repo string) (Project, error) {
	repo = strings.TrimSuffix(strings.Trim(repo, "/"), ".git")
	var r struct {
		FullName string `json:"full_name"`
		Name     string `json:"name"`
	}
	if err := getJSON(githubAPI+"/repos/"+repo, &r); err != nil {
		return Project{}, fmt.Errorf("GitHub repo %s: %w", repo, err)
	}
	return Project{GitHub, r.FullName, r.Name}, nil
}

// Latest returns the newest Forever build of p.
func Latest(p Project) (Release, error) {
	switch p.Source {
	case Curse:
		return curseLatest(p.ID)
	case GitHub:
		return githubLatest(p.ID)
	}
	return Release{}, fmt.Errorf("unknown source %q", p.Source)
}

type curseFile struct {
	ID                 int64    `json:"id"`
	DisplayName        string   `json:"displayName"`
	FileName           string   `json:"fileName"`
	ReleaseType        int      `json:"releaseType"`
	GameVersions       []string `json:"gameVersions"`
	GameVersionTypeIDs []int    `json:"gameVersionTypeIds"`
}

func (f curseFile) forForever() bool {
	if slices.Contains(f.GameVersionTypeIDs, foreverTypeID) {
		return true
	}
	for _, v := range f.GameVersions {
		if strings.HasPrefix(v, foreverVersionPfx) {
			return true
		}
	}
	return false
}

// curseLatest returns the newest Forever release file, falling back to the newest beta/alpha.
// The website API ignores its gameVersionTypeId filter, so files are filtered here.
func curseLatest(id string) (Release, error) {
	const pageSize, maxPages = 50, 4
	var fallback *curseFile
	for page := range maxPages {
		var resp struct {
			Data       []curseFile `json:"data"`
			Pagination struct {
				TotalCount int `json:"totalCount"`
			} `json:"pagination"`
		}
		u := fmt.Sprintf("%s/mods/%s/files?pageIndex=%d&pageSize=%d&sort=dateCreated&sortDescending=true", curseAPI, id, page, pageSize)
		if err := getJSON(u, &resp); err != nil {
			if isStatus(err, http.StatusNotFound) {
				return Release{}, fmt.Errorf("no CurseForge project %s", id)
			}
			return Release{}, err
		}
		for i, f := range resp.Data {
			if !f.forForever() {
				continue
			}
			if f.ReleaseType == curseRelease {
				return curseRel(id, f), nil
			}
			if fallback == nil {
				fallback = &resp.Data[i]
			}
		}
		if (page+1)*pageSize >= resp.Pagination.TotalCount {
			break
		}
	}
	if fallback != nil {
		return curseRel(id, *fallback), nil
	}
	return Release{}, fmt.Errorf("CurseForge project %s has no files for WoW Forever", id)
}

func curseRel(project string, f curseFile) Release {
	v := f.DisplayName
	if v == "" {
		v = f.FileName
	}
	return Release{
		ID:      strconv.FormatInt(f.ID, 10),
		Version: strings.TrimSuffix(v, ".zip"),
		URL:     fmt.Sprintf("%s/mods/%s/files/%d/download", curseAPI, project, f.ID),
	}
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// githubLatest picks the Forever zip from the latest release, using the BigWigs packager's
// release.json when present.
func githubLatest(repo string) (Release, error) {
	var rel struct {
		Tag    string    `json:"tag_name"`
		Assets []ghAsset `json:"assets"`
	}
	if err := getJSON(githubAPI+"/repos/"+repo+"/releases/latest", &rel); err != nil {
		if isStatus(err, http.StatusNotFound) {
			return Release{}, fmt.Errorf("%s has no published releases", repo)
		}
		return Release{}, err
	}
	asset, err := pickGitHubAsset(rel.Assets)
	if err != nil {
		return Release{}, fmt.Errorf("%s %s: %w", repo, rel.Tag, err)
	}
	return Release{ID: rel.Tag, Version: rel.Tag, URL: asset.URL}, nil
}

func pickGitHubAsset(assets []ghAsset) (ghAsset, error) {
	byName := map[string]ghAsset{}
	var zips, forever []ghAsset
	for _, a := range assets {
		byName[a.Name] = a
		if strings.HasSuffix(strings.ToLower(a.Name), ".zip") {
			zips = append(zips, a)
			if strings.Contains(strings.ToLower(a.Name), "forever") {
				forever = append(forever, a)
			}
		}
	}
	if meta, ok := byName["release.json"]; ok {
		var rj struct {
			Releases []struct {
				Filename string `json:"filename"`
				Nolib    bool   `json:"nolib"`
				Metadata []struct {
					Flavor string `json:"flavor"`
				} `json:"metadata"`
			} `json:"releases"`
		}
		if err := getJSON(meta.URL, &rj); err != nil {
			return ghAsset{}, fmt.Errorf("release.json: %w", err)
		}
		for _, r := range rj.Releases {
			for _, m := range r.Metadata {
				if m.Flavor == "forever" && !r.Nolib {
					if a, ok := byName[r.Filename]; ok {
						return a, nil
					}
				}
			}
		}
		return ghAsset{}, errors.New("release.json lists no file for the Forever flavor")
	}
	switch {
	case len(forever) == 1:
		return forever[0], nil
	case len(zips) == 1:
		return zips[0], nil
	case len(zips) == 0:
		return ghAsset{}, errors.New("release has no .zip asset")
	}
	names := make([]string, len(zips))
	for i, a := range zips {
		names[i] = a.Name
	}
	return ghAsset{}, fmt.Errorf("can't tell which asset is for Forever: %s", strings.Join(names, ", "))
}

// WagoSlug is an addon that publishes Forever builds on Wago.
type WagoSlug struct {
	Slug      string
	Downloads int
}

// WagoForeverSlugs lists addons that publish Forever builds on Wago (keyless endpoint).
func WagoForeverSlugs() ([]WagoSlug, error) {
	var resp struct {
		Addons map[string]struct {
			Downloads int `json:"download_count"`
		} `json:"addons"`
	}
	if err := getJSON(wagoAPI+"/api/data/slugs?game_version=forever", &resp); err != nil {
		return nil, err
	}
	out := make([]WagoSlug, 0, len(resp.Addons))
	for slug, a := range resp.Addons {
		out = append(out, WagoSlug{slug, a.Downloads})
	}
	return out, nil
}
