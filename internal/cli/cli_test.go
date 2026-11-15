package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Avicted/wow-addon-manager/internal/install"
	"github.com/Avicted/wow-addon-manager/internal/source"
	"github.com/Avicted/wow-addon-manager/internal/state"
	"github.com/Avicted/wow-addon-manager/internal/toc"
)

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run(args, nil, &out, &errOut, "1.2.3")
	return code, out.String(), errOut.String()
}

func TestVersionAndHelp(t *testing.T) {
	if code, out, _ := run(t, "version"); code != 0 || out != "wam 1.2.3\n" {
		t.Fatalf("version: %d %q", code, out)
	}
	if code, out, _ := run(t, "--version"); code != 0 || out != "wam 1.2.3\n" {
		t.Fatalf("--version: %d %q", code, out)
	}
	if code, out, _ := run(t, "help"); code != 0 || !strings.Contains(out, "Commands:") {
		t.Fatalf("help: %d %q", code, out)
	}
}

func TestErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WAM_DATA_DIR", filepath.Join(dir, "data"))
	if code, _, errOut := run(t, "-addons-dir", filepath.Join(dir, "missing")); code != 1 || !strings.Contains(errOut, "AddOns directory") {
		t.Fatalf("missing dir: %d %q", code, errOut)
	}
	if code, _, errOut := run(t, "-addons-dir", dir, "frobnicate"); code != 2 || !strings.Contains(errOut, "unknown command") {
		t.Fatalf("unknown command: %d %q", code, errOut)
	}
}

func TestTrackAndRemove(t *testing.T) {
	root := t.TempDir()
	addons := filepath.Join(root, "AddOns")
	if err := os.MkdirAll(filepath.Join(addons, "Journalator"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(addons, "Journalator", "Journalator.toc"), []byte("## Version: 171\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	e := &env{addonsDir: addons, out: &out, err: &out,
		lookup: func(string) (source.Project, error) {
			return source.Project{Source: source.Curse, ID: "453921", Title: "Journalator"}, nil
		}}
	e.inst = &install.Installer{AddonsDir: addons, Log: e.logf}
	st, err := state.Load(filepath.Join(root, "data", "state.json"), addons)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmdTrack(e, st, []string{"Journalator", "curse:453921"}); err != nil {
		t.Fatal(err)
	}
	if a := st.Find("Journalator"); a == nil || a.Version != "171" || a.Ref() != "curse:453921" {
		t.Fatalf("tracked %+v", a)
	}
	if err := cmdRemove(e, st, []string{"journalator"}); err != nil {
		t.Fatal(err)
	}
	if st.Find("Journalator") != nil {
		t.Fatal("still tracked after remove")
	}
	if _, err := os.Stat(filepath.Join(addons, "Journalator")); !os.IsNotExist(err) {
		t.Fatal("folder still in AddOns")
	}
}

// fixture is an AddOns directory, a state and fake sources: releases by project ref, zips by
// release URL, and projects by what the user types.
type fixture struct {
	e        *env
	st       *state.State
	out, log *bytes.Buffer
	addons   string
	root     string
	releases map[string]source.Release
	zips     map[string]map[string]string
	projects map[string]source.Project
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{
		out: &bytes.Buffer{}, log: &bytes.Buffer{}, root: root, addons: filepath.Join(root, "Interface", "AddOns"),
		releases: map[string]source.Release{}, zips: map[string]map[string]string{}, projects: map[string]source.Project{},
	}
	if err := os.MkdirAll(f.addons, 0o755); err != nil {
		t.Fatal(err)
	}
	f.e = &env{addonsDir: f.addons, out: f.out, err: f.log}
	f.e.lookup = func(ref string) (source.Project, error) {
		if p, ok := f.projects[ref]; ok {
			return p, nil
		}
		return source.Project{}, fmt.Errorf("unknown ref %q", ref)
	}
	f.e.latest = func(p source.Project) (source.Release, error) {
		if r, ok := f.releases[p.Ref()]; ok {
			return r, nil
		}
		return source.Release{}, fmt.Errorf("no release for %s", p.Ref())
	}
	f.e.inst = &install.Installer{AddonsDir: f.addons, Log: f.e.logf,
		Download: func(url, dst string) error {
			files, ok := f.zips[url]
			if !ok {
				return fmt.Errorf("download %s: not found", url)
			}
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			for name, body := range files {
				w, err := zw.Create(name)
				if err != nil {
					return err
				}
				if _, err := w.Write([]byte(body)); err != nil {
					return err
				}
			}
			if err := zw.Close(); err != nil {
				return err
			}
			return os.WriteFile(dst, buf.Bytes(), 0o644)
		}}
	st, err := state.Load(filepath.Join(root, "data", "state.json"), f.addons)
	if err != nil {
		t.Fatal(err)
	}
	f.st = st
	return f
}

// folder creates an AddOns folder with a .toc holding the given version.
func (f *fixture) folder(t *testing.T, name, version string) {
	t.Helper()
	dir := filepath.Join(f.addons, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".toc"), []byte("## Version: "+version+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// track records an addon wam installed (release set) or adopted (release empty), with a
// matching folder, and gives its project a latest release.
func (f *fixture) track(t *testing.T, name, id, installed, latestID string) *state.Addon {
	t.Helper()
	f.folder(t, name, installed)
	a := &state.Addon{Name: name, Source: source.Curse, Project: id, Version: installed, Folders: []string{name}}
	if latestID != "" {
		url := "zip://" + name
		f.releases[a.Ref()] = source.Release{ID: latestID, Version: latestID, URL: url}
		f.zips[url] = map[string]string{name + "/" + name + ".toc": "## Version: " + latestID + "\n"}
	}
	f.st.Addons = append(f.st.Addons, a)
	return a
}

func (f *fixture) version(t *testing.T, folder string) string {
	t.Helper()
	tf, err := toc.Read(f.addons, folder)
	if err != nil {
		t.Fatal(err)
	}
	return tf.Get("Version")
}

func TestStatus(t *testing.T) {
	f := newFixture(t)
	f.track(t, "Questie", "1", "10", "10").Release = "10"
	f.track(t, "Auctionator", "2", "341", "346") // adopted: Release empty
	f.track(t, "Details", "3", "5", "")          // no release known: lookup fails
	f.folder(t, "Mystery", "1")                  // not tracked

	if err := cmdStatus(f.e, f.st, nil); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	for _, want := range []string{
		"Auctionator  341 ~", "update", "error", "Questie", "ok",
		"Errors:", "no release for curse:3",
		"Not tracked", "Mystery", "wam track Mystery",
		"~ installed outside wam", "1 update available. Run: wam update",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("status output lacks %q:\n%s", want, out)
		}
	}
	if err := cmdStatus(f.e, f.st, []string{"extra"}); err == nil {
		t.Error("status with arguments: want error")
	}
}

func TestStatusAllUpToDate(t *testing.T) {
	f := newFixture(t)
	f.track(t, "Questie", "1", "10", "10").Release = "10"
	if err := cmdStatus(f.e, f.st, nil); err != nil {
		t.Fatal(err)
	}
	if out := f.out.String(); !strings.Contains(out, "All 1 addon up to date.") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestUpdateDryRunChangesNothing(t *testing.T) {
	f := newFixture(t)
	a := f.track(t, "Questie", "1", "10", "11")
	a.Release = "10"
	if err := cmdUpdate(f.e, f.st, []string{"-n"}); err != nil {
		t.Fatal(err)
	}
	if out := f.out.String(); !strings.Contains(out, "would update") || !strings.Contains(out, "Dry run: 1 update") {
		t.Fatalf("output:\n%s", out)
	}
	if v := f.version(t, "Questie"); v != "10" || a.Release != "10" {
		t.Fatalf("dry run changed things: toc %q, release %q", v, a.Release)
	}
}

func TestUpdateInstallsAndSaves(t *testing.T) {
	f := newFixture(t)
	a := f.track(t, "Questie", "1", "10", "11")
	a.Release = "10"
	if err := cmdUpdate(f.e, f.st, nil); err != nil {
		t.Fatal(err)
	}
	if v := f.version(t, "Questie"); v != "11" || a.Release != "11" {
		t.Fatalf("not updated: toc %q, release %q", v, a.Release)
	}
	saved, err := state.Load(filepath.Join(f.root, "data", "state.json"), f.addons)
	if err != nil || saved.Find("Questie") == nil || saved.Find("Questie").Release != "11" {
		t.Fatalf("state not saved: %+v, %v", saved, err)
	}
}

func TestUpdateOnlyNamedAddons(t *testing.T) {
	f := newFixture(t)
	f.track(t, "Questie", "1", "10", "11").Release = "10"
	f.track(t, "Details", "2", "5", "6").Release = "5"
	if err := cmdUpdate(f.e, f.st, []string{"questie"}); err != nil {
		t.Fatal(err)
	}
	if f.version(t, "Questie") != "11" || f.version(t, "Details") != "5" {
		t.Fatalf("Questie %q, Details %q", f.version(t, "Questie"), f.version(t, "Details"))
	}
	if err := cmdUpdate(f.e, f.st, []string{"Nope"}); err == nil || !strings.Contains(err.Error(), "not a tracked addon") {
		t.Fatalf("unknown name: %v", err)
	}
}

func TestUpdateFailureSetsError(t *testing.T) {
	f := newFixture(t)
	a := f.track(t, "Questie", "1", "10", "11")
	a.Release = "10"
	delete(f.zips, "zip://Questie") // the download will fail
	err := cmdUpdate(f.e, f.st, nil)
	if err == nil || !strings.Contains(err.Error(), "1 addon(s) failed") {
		t.Fatalf("err = %v", err)
	}
	if f.version(t, "Questie") != "10" || a.Release != "10" {
		t.Fatal("failed update changed the addon")
	}
}

func TestInstallNewAddon(t *testing.T) {
	f := newFixture(t)
	p := source.Project{Source: source.Curse, ID: "9", Title: "Details: Damage Meter"}
	f.projects["details"] = p
	f.releases[p.Ref()] = source.Release{ID: "3", Version: "3.0", URL: "zip://details"}
	f.zips["zip://details"] = map[string]string{"Details/Details.toc": "## Version: 3.0\n", "Details_Plugin/Details_Plugin.toc": "x"}
	if err := cmdInstall(f.e, f.st, []string{"details"}); err != nil {
		t.Fatal(err)
	}
	a := f.st.Find("Details")
	if a == nil || a.Release != "3" || len(a.Folders) != 2 {
		t.Fatalf("tracked %+v", a)
	}
	if f.version(t, "Details") != "3.0" {
		t.Fatal("folder not installed")
	}
}

func TestInstallAlreadyInstalled(t *testing.T) {
	f := newFixture(t)
	a := f.track(t, "Questie", "1", "10", "10")
	a.Release = "10"
	f.projects["questie"] = source.Project{Source: source.Curse, ID: "1", Title: "Questie"}
	err := cmdInstall(f.e, f.st, []string{"questie"})
	if err == nil || !strings.Contains(err.Error(), "already installed") {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallFailureLeavesNoEntry(t *testing.T) {
	f := newFixture(t)
	p := source.Project{Source: source.Curse, ID: "9", Title: "Details"}
	f.projects["details"] = p
	f.releases[p.Ref()] = source.Release{ID: "3", Version: "3.0", URL: "zip://missing"}
	if err := cmdInstall(f.e, f.st, []string{"details"}); err == nil {
		t.Fatal("want download error")
	}
	if f.st.Find("Details") != nil || len(f.st.Addons) != 0 {
		t.Fatalf("state after failed install: %+v", f.st.Addons)
	}
}

func TestUpdateStopsWhenInterrupted(t *testing.T) {
	f := newFixture(t)
	f.track(t, "Questie", "1", "10", "11").Release = "10"
	f.track(t, "Details", "2", "5", "6").Release = "5"
	ctx, cancel := context.WithCancel(context.Background())
	f.e.ctx = ctx
	download := f.e.inst.Download
	// Ctrl-C arrives while the first addon downloads: it finishes, the second is skipped.
	f.e.inst.Download = func(url, dst string) error { cancel(); return download(url, dst) }

	err := cmdUpdate(f.e, f.st, nil)
	if err == nil || !strings.Contains(err.Error(), "interrupted: 1 addon not updated") {
		t.Fatalf("err = %v", err)
	}
	if f.version(t, "Questie") != "11" || f.version(t, "Details") != "5" {
		t.Fatalf("Questie %q, Details %q", f.version(t, "Questie"), f.version(t, "Details"))
	}
	if out := f.out.String(); !strings.Contains(out, "Updated 1 addon, 1 skipped.") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestRunRefusesWhileAnotherWamRuns(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "data")
	t.Setenv("WAM_DATA_DIR", data)
	unlock, err := state.Lock(data)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if code, _, errOut := run(t, "-addons-dir", dir, "status"); code != 1 || !strings.Contains(errOut, "another wam is running") {
		t.Fatalf("%d %q", code, errOut)
	}
}

func TestStatusSurvivesAnUnreadableFolder(t *testing.T) {
	f := newFixture(t)
	f.track(t, "Questie", "1", "10", "10").Release = "10"
	if err := os.MkdirAll(filepath.Join(f.addons, "Bad"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(f.addons, "nowhere"), filepath.Join(f.addons, "Bad", "Bad.toc")); err != nil {
		t.Fatal(err)
	}
	if err := cmdStatus(f.e, f.st, nil); err != nil {
		t.Fatal(err)
	}
	if out := f.out.String(); !strings.Contains(out, "Questie") || strings.Contains(out, "wam track Bad") {
		t.Fatalf("output:\n%s", out)
	}
	if !strings.Contains(f.log.String(), "warning: can't read Bad") {
		t.Fatalf("log:\n%s", f.log.String())
	}
}

func TestSearch(t *testing.T) {
	f := newFixture(t)
	if err := cmdSearch(f.e, f.st, nil); err == nil {
		t.Error("search without text should be a usage error")
	}

	known := map[string]source.Project{
		"details":  {Source: source.Curse, ID: "1", Title: "Details"},
		"detailsx": {Source: source.Curse, ID: "2", Title: "Details Extra"},
	}
	f.e.bySlug = func(slug string) (source.Project, error) {
		if p, ok := known[slug]; ok {
			return p, nil
		}
		return source.Project{}, fmt.Errorf("not found")
	}
	f.e.forever = func() ([]source.WagoSlug, error) {
		return []source.WagoSlug{{Slug: "detailsx", Downloads: 5}, {Slug: "details-wago", Downloads: 50}, {Slug: "other", Downloads: 99}}, nil
	}
	if err := cmdSearch(f.e, f.st, []string{"details"}); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	for _, want := range []string{"Details", "Details Extra", "wam install curse:2", "details-wago", "not matched on CurseForge"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "other") {
		t.Errorf("non-matching slug listed:\n%s", out)
	}
	if strings.Index(out, "details-wago") > strings.Index(out, "Details Extra") {
		t.Errorf("results not ordered by downloads:\n%s", out)
	}
}

func TestSearchWithoutWago(t *testing.T) {
	f := newFixture(t)
	f.e.bySlug = func(slug string) (source.Project, error) {
		if slug == "details" {
			return source.Project{Source: source.Curse, ID: "1", Title: "Details"}, nil
		}
		return source.Project{}, fmt.Errorf("not found")
	}
	f.e.forever = func() ([]source.WagoSlug, error) { return nil, fmt.Errorf("down") }
	if err := cmdSearch(f.e, f.st, []string{"details"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), "Details") || !strings.Contains(f.log.String(), "Wago addon list unavailable") {
		t.Errorf("out:\n%s\nlog:\n%s", f.out, f.log)
	}

	f.out.Reset()
	if err := cmdSearch(f.e, f.st, []string{"zzz"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), "Nothing found") {
		t.Errorf("out:\n%s", f.out)
	}
}

// events parses -json output, one object per line.
func events(t *testing.T, out string) []map[string]any {
	t.Helper()
	var evs []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("not JSON: %q: %v", line, err)
		}
		evs = append(evs, ev)
	}
	return evs
}

func TestJSONStatus(t *testing.T) {
	f := newFixture(t)
	f.e.json = true
	f.track(t, "Questie", "1", "10", "11").Release = "10"
	f.track(t, "Details", "2", "5", "") // no release: its check fails
	f.folder(t, "Journalator", "171")
	f.st.Unresolved = []string{"Journalator"}
	if err := cmdStatus(f.e, f.st, nil); err != nil {
		t.Fatal(err)
	}
	evs := events(t, f.out.String())
	if evs[0]["event"] != "checking" || len(evs[0]["addons"].([]any)) != 2 {
		t.Fatalf("first event %v", evs[0])
	}
	checked := 0
	for _, ev := range evs[1:] {
		if ev["event"] == "checked" {
			checked++
			if ev["total"] != 2.0 || ev["done"] != float64(checked) {
				t.Errorf("checked event %v", ev)
			}
		}
	}
	if checked != 2 {
		t.Fatalf("%d checked events in %v", checked, evs)
	}
	r := f.e.data.(statusReport)
	if r.Updates != 1 || len(r.Untracked) != 1 || r.Untracked[0].Root != "Journalator" || r.BackupsDir == "" {
		t.Fatalf("report %+v", r)
	}
	for _, a := range r.Addons {
		if a.Name == "Details" && (a.Status != "error" || a.Error == nil || a.Error.Code != "other") {
			t.Errorf("Details row %+v", a)
		}
	}
}

func TestJSONUpdateReportsEachAddon(t *testing.T) {
	f := newFixture(t)
	f.e.json = true
	f.track(t, "Questie", "1", "10", "11").Release = "10"
	f.track(t, "Details", "2", "5", "6").Release = "5"
	f.track(t, "Bagnon", "3", "7", "8").Release = "7"
	delete(f.zips, "zip://Details")
	ctx, cancel := context.WithCancel(context.Background())
	f.e.ctx = ctx
	download := f.e.inst.Download
	// Details fails to download, and Cancel arrives while it does: Bagnon is skipped.
	f.e.inst.Download = func(url, dst string) error {
		if url == "zip://Details" {
			cancel()
		}
		return download(url, dst)
	}
	if err := cmdUpdate(f.e, f.st, nil); err == nil || !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("err = %v", err)
	}
	states := map[string][]string{}
	for _, ev := range events(t, f.out.String()) {
		if ev["event"] == "item" {
			it := ev["item"].(map[string]any)
			states[it["name"].(string)] = append(states[it["name"].(string)], it["state"].(string))
		}
	}
	want := map[string]string{"Questie": "waiting updating updated", "Details": "waiting updating failed", "Bagnon": "waiting skipped"}
	for name, w := range want {
		if got := strings.Join(states[name], " "); got != w {
			t.Errorf("%s: states %q, want %q", name, got, w)
		}
	}
	r := f.e.data.(*updateReport)
	if r.Updated != 1 || r.Failed != 1 || r.Skipped != 1 || r.Items[1].Error == nil {
		t.Fatalf("report %+v", r)
	}
}

func TestJSONSearch(t *testing.T) {
	f := newFixture(t)
	f.e.json = true
	f.e.bySlug = func(slug string) (source.Project, error) {
		if slug == "details" {
			return source.Project{Source: source.Curse, ID: "1", Title: "Details"}, nil
		}
		return source.Project{}, fmt.Errorf("not found")
	}
	f.e.forever = func() ([]source.WagoSlug, error) {
		return []source.WagoSlug{{Slug: "details-wago", Downloads: 50}}, nil
	}
	if err := cmdSearch(f.e, f.st, []string{"details"}); err != nil {
		t.Fatal(err)
	}
	r := f.e.data.(searchReport)
	if len(r.Hits) != 2 || r.Hits[0].Downloads != nil || r.Hits[0].Ref != "curse:1" ||
		r.Hits[1].Ref != "" || *r.Hits[1].Downloads != 50 || !strings.HasSuffix(r.Hits[1].URL, "/details-wago") {
		t.Fatalf("report %+v", r)
	}
	if f.out.Len() != 0 {
		t.Fatalf("search printed text with -json: %q", f.out)
	}
}

func TestJSONRunErrors(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "data")
	t.Setenv("WAM_DATA_DIR", data)
	missing := filepath.Join(dir, "missing")
	code, out, _ := run(t, "-json", "-addons-dir", missing, "status")
	ev := events(t, out)
	if e := ev[0]["error"].(map[string]any); code != 1 || len(ev) != 1 || ev[0]["ok"] != false || e["code"] != "addons_dir" || e["path"] != missing {
		t.Fatalf("missing dir: %d %v", code, ev)
	}
	unlock, err := state.Lock(data)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	code, out, _ = run(t, "-json", "-addons-dir", dir, "status")
	if e := events(t, out)[0]["error"].(map[string]any); code != 1 || e["code"] != "locked" {
		t.Fatalf("locked: %d %s", code, out)
	}
	if _, out, _ = run(t, "-json", "version"); !strings.Contains(out, `"data":{"version":"1.2.3"}`) {
		t.Fatalf("version: %s", out)
	}
}

func TestWatchCancel(t *testing.T) {
	for in, want := range map[string]bool{"noise\n cancel \n": true, "noise\n": false, "": false} {
		called := false
		watchCancel(strings.NewReader(in), func() { called = true })
		if called != want {
			t.Errorf("%q: cancelled %v, want %v", in, called, want)
		}
	}
}

func TestDescribe(t *testing.T) {
	cases := map[string]error{
		"not_found":  fmt.Errorf("no CurseForge project 1: %w", source.ErrNotFound),
		"network":    &url.Error{Op: "Get", URL: "https://example.com", Err: errors.New("dial tcp: no route")},
		"permission": &fs.PathError{Op: "rename", Path: "x", Err: fs.ErrPermission},
		"locked":     fmt.Errorf("%w (it holds x)", state.ErrLocked),
		"cancelled":  fmt.Errorf("%w: 2 addons not updated", errInterrupted),
		"bad_ref":    badRef{fmt.Errorf("no CurseForge addon with slug %q: %w", "zz", source.ErrNotFound)},
		"other":      errors.New("boom"),
	}
	for want, err := range cases {
		if got := describe(err); got.Code != want || got.Message != err.Error() {
			t.Errorf("%v: %+v, want code %s", err, got, want)
		}
	}
	if describe(nil) != nil {
		t.Error("describe(nil) != nil")
	}
}

func TestStatusNotesOldBackups(t *testing.T) {
	f := newFixture(t)
	f.e.dataDir = filepath.Join(f.root, "data")

	if err := cmdStatus(f.e, f.st, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(f.log.String(), "wam 1.0") {
		t.Errorf("note without old backups:\n%s", f.log)
	}

	if err := os.MkdirAll(filepath.Join(f.e.dataDir, "backups", "Questie"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cmdStatus(f.e, f.st, nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"backups made by wam 1.0", filepath.Join(f.e.dataDir, "backups"), f.e.inst.BackupsDir()} {
		if !strings.Contains(f.log.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, f.log)
		}
	}
}
