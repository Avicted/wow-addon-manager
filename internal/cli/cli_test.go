package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Avicted/wow-addon-manager/internal/install"
	"github.com/Avicted/wow-addon-manager/internal/source"
	"github.com/Avicted/wow-addon-manager/internal/state"
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
