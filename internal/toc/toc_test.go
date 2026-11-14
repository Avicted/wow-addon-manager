package toc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestParse(t *testing.T) {
	in := "\ufeff## Interface: 16001\r\n## Title: |cff00ff00Questie|r v1\r\n##Version:12.6.4\r\n## Version: dup\r\n# comment\r\nfile.lua\r\n"
	got := Parse(strings.NewReader(in))
	if got["interface"] != "16001" || got["version"] != "12.6.4" || CleanTitle(got["title"]) != "Questie v1" {
		t.Fatalf("Parse = %v", got)
	}
}

func TestParseReadsOnlyHeader(t *testing.T) {
	huge := "## Version: 1.0.9\n" + strings.Repeat("## X-Quest: "+strings.Repeat("A", 1000)+"\n", 100000)
	if got := Parse(strings.NewReader(huge)); got["version"] != "1.0.9" {
		t.Fatalf("version = %q", got["version"])
	}
}

func TestReadPicksForeverTOC(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Questie", "Questie.toc"), "## Version: plain\n")
	writeFile(t, filepath.Join(dir, "Questie", "Questie_Mists.toc"), "## Version: mists\n")
	writeFile(t, filepath.Join(dir, "Questie", "Questie_Forever.toc"), "## Version: forever\n")
	writeFile(t, filepath.Join(dir, "Guide", "Guide-Vanilla.toc"), "## Version: vanilla\n")
	writeFile(t, filepath.Join(dir, "Guide", "Guide-Camelot.toc"), "## Version: camelot\n")
	for folder, want := range map[string]string{"Questie": "forever", "Guide": "camelot"} {
		f, err := Read(dir, folder)
		if err != nil || f.Get("Version") != want || f.Broken {
			t.Errorf("%s: version %q broken %v err %v, want %q", folder, f.Get("Version"), f.Broken, err, want)
		}
	}
}

func TestReadBroken(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "AL", `A\A.toc`), "## Version: 1\n")
	writeFile(t, filepath.Join(dir, "AL", `B\B.toc`), "## X-Curse-Project-ID: 1422985\n")
	f, err := Read(dir, "AL")
	if err != nil || !f.Broken || f.Get("X-Curse-Project-ID") != "1422985" {
		t.Fatalf("got %+v, %v", f, err)
	}
}

func TestScanSkipsHiddenAndFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "A", "A.toc"), "## Version: 1\n")
	writeFile(t, filepath.Join(dir, ".wam-staging-1", "B", "B.toc"), "")
	writeFile(t, filepath.Join(dir, "notes.txt"), "")
	got, err := Scan(dir)
	if err != nil || len(got) != 1 || got[0].Name != "A" {
		t.Fatalf("Scan = %+v, %v", got, err)
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{
		"## Title: X\n## Version: 1\n", "\ufeff## Title: X\r\n", "##:\n## : x\n###: y\n", "no header\n", "",
		"## Title: " + strings.Repeat("a", readLimit*2),
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		fields := Parse(strings.NewReader(s))
		total := 0
		for k, v := range fields {
			if k != strings.ToLower(k) || strings.TrimSpace(k) != k || k == "" {
				t.Fatalf("bad key %q", k)
			}
			total += len(k) + len(v)
		}
		if total > readLimit {
			t.Fatalf("parsed %d bytes of fields from a limit of %d", total, readLimit)
		}
	})
}

func TestScanKeepsUnreadableFolders(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Good", "Good.toc"), "## Version: 1\n")
	// A dangling symlink: the .toc is listed but can't be opened (works the same as root).
	if err := os.MkdirAll(filepath.Join(dir, "Bad"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "nowhere"), filepath.Join(dir, "Bad", "Bad.toc")); err != nil {
		t.Fatal(err)
	}
	got, err := Scan(dir)
	if err != nil || len(got) != 2 {
		t.Fatalf("Scan = %+v, %v", got, err)
	}
	if got[0].Name != "Bad" || got[0].Err == nil || got[1].Name != "Good" || got[1].Err != nil || got[1].Get("Version") != "1" {
		t.Fatalf("Scan = %+v", got)
	}
}
