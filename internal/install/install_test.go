package install

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Avicted/wow-addon-manager/internal/source"
	"github.com/Avicted/wow-addon-manager/internal/state"
	"github.com/Avicted/wow-addon-manager/internal/toc"
)

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// release serves a zip and returns a Release pointing at it.
func release(t *testing.T, id string, files map[string]string) source.Release {
	t.Helper()
	body := makeZip(t, files)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) }))
	t.Cleanup(srv.Close)
	return source.Release{ID: id, Version: id, URL: srv.URL}
}

func version(t *testing.T, dir, folder string) string {
	t.Helper()
	f, err := toc.Read(dir, folder)
	if err != nil {
		t.Fatal(err)
	}
	return f.Get("Version")
}

func newInstaller(t *testing.T) *Installer {
	root := t.TempDir()
	in := &Installer{AddonsDir: filepath.Join(root, "Interface", "AddOns"), DataDir: filepath.Join(root, "data"), Log: t.Logf}
	if err := os.MkdirAll(in.AddonsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return in
}

func TestExtractZip(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		want    []string
		wantErr string
		check   string
	}{
		{"plain", map[string]string{"A/A.toc": "x", "A_Sub/A_Sub.toc": "x", "A/libs/l.lua": "x"}, []string{"A", "A_Sub"}, "", "A/libs/l.lua"},
		{"backslashes", map[string]string{`AtlasLootClassic\AtlasLootClassic.toc`: "x", `AtlasBIS\db\x.lua`: "x"}, []string{"AtlasBIS", "AtlasLootClassic"}, "", "AtlasBIS/db/x.lua"},
		{"wrapper", map[string]string{"Pack-1.0/A/A.toc": "x", "Pack-1.0/B/B.toc": "x"}, []string{"A", "B"}, "", "B/B.toc"},
		{"single folder kept", map[string]string{"A/A.toc": "x", "A/Sub/S.toc": "x"}, []string{"A"}, "", "A/Sub/S.toc"},
		{"traversal", map[string]string{"A/../../evil.lua": "x"}, nil, "unsafe", ""},
		{"absolute", map[string]string{"/etc/evil": "x"}, nil, "absolute", ""},
		{"root file", map[string]string{"A.toc": "x"}, nil, "outside any addon folder", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			zp := filepath.Join(t.TempDir(), "a.zip")
			if err := os.WriteFile(zp, makeZip(t, c.files), 0o644); err != nil {
				t.Fatal(err)
			}
			dest := t.TempDir()
			got, err := ExtractZip(zp, dest)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want %q", err, c.wantErr)
				}
				return
			}
			if err != nil || !slices.Equal(got, c.want) {
				t.Fatalf("got %v, %v; want %v", got, err, c.want)
			}
			if !exists(filepath.Join(dest, c.check)) {
				t.Fatalf("%s not extracted", c.check)
			}
		})
	}
}

func TestInstallSwapsAndBacksUp(t *testing.T) {
	in := newInstaller(t)
	writeFile(t, filepath.Join(in.AddonsDir, "Foo", "Foo.toc"), "## Version: 1\n")
	writeFile(t, filepath.Join(in.AddonsDir, "Foo", "stale.lua"), "old")
	writeFile(t, filepath.Join(in.AddonsDir, "Other", "Other.toc"), "## Version: 1\n")
	rel := release(t, "7", map[string]string{`Foo\Foo.toc`: "## Version: 2\n", "Foo_Ext/Foo_Ext.toc": "x"})

	a := &state.Addon{Name: "Foo", Source: source.Curse, Project: "42", Version: "1", Folders: []string{"Foo"}}
	st := &state.State{AddonsDir: in.AddonsDir, Addons: []*state.Addon{a}}
	if err := in.Install(st, a, rel, false); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(in.AddonsDir, "Foo", "stale.lua")) || !exists(filepath.Join(in.AddonsDir, "Foo_Ext", "Foo_Ext.toc")) {
		t.Fatal("folders not swapped")
	}
	if v := version(t, in.AddonsDir, "Foo"); v != "2" {
		t.Fatalf("new TOC version %q", v)
	}
	if !exists(filepath.Join(in.AddonsDir, "Other")) {
		t.Fatal("unrelated folder touched")
	}
	backups, _ := filepath.Glob(filepath.Join(in.DataDir, "backups", "Foo", "*", "Foo", "stale.lua"))
	if len(backups) != 1 {
		t.Fatalf("backup missing: %v", backups)
	}
	if a.Release != "7" || !slices.Equal(a.Folders, []string{"Foo", "Foo_Ext"}) {
		t.Fatalf("state not updated: %+v", a)
	}
	entries, _ := os.ReadDir(filepath.Dir(in.AddonsDir))
	for _, en := range entries {
		if strings.HasPrefix(en.Name(), ".wam-staging-") {
			t.Fatal("staging dir left behind")
		}
	}
}

func TestInstallFolderOwnedByAnotherAddon(t *testing.T) {
	in := newInstaller(t)
	writeFile(t, filepath.Join(in.AddonsDir, "Shared", "Shared.toc"), "## Version: mine\n")
	rel := release(t, "7", map[string]string{"Shared/Shared.toc": "## Version: new\n"})
	owner := &state.Addon{Name: "Owner", Source: source.Curse, Project: "1", Release: "1", Folders: []string{"Shared"}}
	a := &state.Addon{Name: "New", Source: source.Curse, Project: "42"}
	st := &state.State{AddonsDir: in.AddonsDir, Addons: []*state.Addon{owner, a}}

	// update: refuse
	err := in.Install(st, a, rel, false)
	if err == nil || !strings.Contains(err.Error(), "belongs to Owner") {
		t.Fatalf("err = %v", err)
	}
	if version(t, in.AddonsDir, "Shared") != "mine" || a.Release != "" {
		t.Fatal("folder or state changed")
	}

	// explicit install: take over, and drop the previous owner that has nothing left
	if err := in.Install(st, a, rel, true); err != nil {
		t.Fatal(err)
	}
	if version(t, in.AddonsDir, "Shared") != "new" || st.Owner("Shared") != a {
		t.Fatal("folder not taken over")
	}
	if st.Find("Owner") != nil || st.Find("New") != a {
		t.Fatalf("addons after takeover: %+v", st.Addons)
	}
}

func TestReinstallKeepsAddonTracked(t *testing.T) {
	in := newInstaller(t)
	writeFile(t, filepath.Join(in.AddonsDir, "Foo", "Foo.toc"), "## Version: 1\n")
	rel := release(t, "7", map[string]string{"Foo/Foo.toc": "## Version: 2\n"})
	a := &state.Addon{Name: "Foo", Source: source.Curse, Project: "42", Folders: []string{"Foo"}}
	st := &state.State{AddonsDir: in.AddonsDir, Addons: []*state.Addon{a}}
	if err := in.Install(st, a, rel, true); err != nil {
		t.Fatal(err)
	}
	if st.Find("Foo") != a || !slices.Equal(a.Folders, []string{"Foo"}) {
		t.Fatalf("addon lost from state: %+v", st.Addons)
	}
}

func TestInstallRollsBackOnFailure(t *testing.T) {
	in := newInstaller(t)
	writeFile(t, filepath.Join(in.AddonsDir, "Foo", "Foo.toc"), "## Version: 1\n")
	writeFile(t, filepath.Join(in.AddonsDir, "Foo_Ext", "Foo_Ext.toc"), "## Version: 1\n")
	rel := release(t, "7", map[string]string{"Foo/Foo.toc": "## Version: 2\n", "Foo_Ext/Foo_Ext.toc": "## Version: 2\n"})
	// Moving the second new folder into AddOns fails, after both old folders were backed up
	// and the first new one was moved in.
	moves := 0
	move = func(src, dst string) error {
		moves++
		if moves == 4 {
			return fmt.Errorf("disk full")
		}
		return moveDir(src, dst)
	}
	t.Cleanup(func() { move = moveDir })

	a := &state.Addon{Name: "Foo", Source: source.Curse, Project: "42", Version: "1", Folders: []string{"Foo", "Foo_Ext"}}
	st := &state.State{AddonsDir: in.AddonsDir, Addons: []*state.Addon{a}}
	err := in.Install(st, a, rel, false)
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("err = %v", err)
	}
	for _, f := range []string{"Foo", "Foo_Ext"} {
		if v := version(t, in.AddonsDir, f); v != "1" {
			t.Errorf("%s has version %q after rollback, want 1", f, v)
		}
	}
	if a.Release != "" || a.Version != "1" {
		t.Errorf("state changed after failed install: %+v", a)
	}
}

func TestRemoveMovesToBackup(t *testing.T) {
	in := newInstaller(t)
	writeFile(t, filepath.Join(in.AddonsDir, "Foo", "Foo.toc"), "x")
	backup, err := in.Remove("Foo", []string{"Foo", "Missing"})
	if err != nil || exists(filepath.Join(in.AddonsDir, "Foo")) || !exists(filepath.Join(backup, "Foo", "Foo.toc")) {
		t.Fatalf("backup %s, err %v", backup, err)
	}
}

func TestCleanStagingRemovesOnlyOldStagingDirs(t *testing.T) {
	in := newInstaller(t)
	parent := filepath.Dir(in.AddonsDir)
	for _, d := range []string{".wam-staging-old", ".wam-staging-new", "Other"} {
		writeFile(t, filepath.Join(parent, d, "f"), "x")
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(filepath.Join(parent, ".wam-staging-old"), old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(parent, "Other"), old, old); err != nil {
		t.Fatal(err)
	}
	CleanStaging(in.AddonsDir, time.Hour)
	if exists(filepath.Join(parent, ".wam-staging-old")) {
		t.Error("old staging directory kept")
	}
	if !exists(filepath.Join(parent, ".wam-staging-new")) || !exists(filepath.Join(parent, "Other")) || !exists(in.AddonsDir) {
		t.Error("removed something it shouldn't have")
	}
}

// FuzzExtractZip checks that whatever an archive calls its files, nothing is written outside
// the destination and nothing panics.
func FuzzExtractZip(f *testing.F) {
	for _, name := range []string{
		"A/A.toc", `AtlasLoot\AtlasLoot.toc`, "Pack/A/A.toc", "A/../../evil.lua", "/etc/evil", `C:\evil`,
		`..\evil`, "A/./B/../C.toc", "A//B.toc", "__MACOSX/A/._A.toc", "A/", "", ".", "..", "A/\x00B",
	} {
		f.Add(name, "body")
	}
	f.Fuzz(func(t *testing.T, name, body string) {
		if strings.HasSuffix(name, "/") {
			body = "" // a directory entry has no content
		}
		root := t.TempDir()
		dest := filepath.Join(root, "dest")
		if err := os.Mkdir(dest, 0o755); err != nil {
			t.Fatal(err)
		}
		zp := filepath.Join(root, "a.zip")
		if err := os.WriteFile(zp, makeZip(t, map[string]string{name: body, "Keep/Keep.toc": "x"}), 0o644); err != nil {
			t.Fatal(err)
		}
		_, _ = ExtractZip(zp, dest)

		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Name() != "dest" && e.Name() != "a.zip" {
				t.Fatalf("%q wrote %s outside the destination", name, e.Name())
			}
		}
	})
}
