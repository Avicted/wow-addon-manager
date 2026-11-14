package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
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
	code := Run(args, &out, &errOut, "1.2.3")
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
	e.inst = &install.Installer{AddonsDir: addons, DataDir: filepath.Join(root, "data"), Log: e.logf}
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
	releases map[string]source.Release
	zips     map[string]map[string]string
	projects map[string]source.Project
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{
		out: &bytes.Buffer{}, log: &bytes.Buffer{}, addons: filepath.Join(root, "Interface", "AddOns"),
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
	f.e.inst = &install.Installer{AddonsDir: f.addons, DataDir: filepath.Join(root, "data"), Log: f.e.logf,
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
	saved, err := state.Load(filepath.Join(filepath.Dir(f.e.inst.DataDir), "data", "state.json"), f.addons)
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
