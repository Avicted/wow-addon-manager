package state

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Avicted/wow-addon-manager/internal/source"
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

func scan(t *testing.T, dir string) []toc.Folder {
	t.Helper()
	folders, err := toc.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	return folders
}

func TestAdopt(t *testing.T) {
	d := t.TempDir()
	writeFile(t, filepath.Join(d, "Questie", "Questie_Forever.toc"), "## Version: 12.6.4\n## X-Curse-Project-ID: 334372\n")
	writeFile(t, filepath.Join(d, "QuestieDB", "QuestieDB_Forever.toc"), "## Version: 1.0.9\n")
	writeFile(t, filepath.Join(d, "ForeverArtisan_Core", "ForeverArtisan_Core.toc"), "## Version: 0.9\n## X-Curse-Project-ID: 1716370\n")
	writeFile(t, filepath.Join(d, "ForeverArtisan_Alchemy", "ForeverArtisan_Alchemy.toc"), "## Version: 0.9\n")
	writeFile(t, filepath.Join(d, "BetterFishing", "BetterFishing.toc"), "## Title: |cffEEE4AEBetter Fishing|r\n## Version: 65\n")
	writeFile(t, filepath.Join(d, "Auctionator", "Auctionator.toc"), "## Version: 341\n## X-Website: https://www.curseforge.com/wow/addons/auctionator\n")
	writeFile(t, filepath.Join(d, "Guidelime", "Guidelime-Camelot.toc"), "## Title: Guidelime\n## Version: 6\n## Dependencies: Questie\n")
	writeFile(t, filepath.Join(d, "Journalator", "Journalator.toc"), "## Version: 171\n")
	writeFile(t, filepath.Join(d, "Journalator_Display", "Journalator_Display.toc"), "## RequiredDeps: Auctionator, Journalator\n")
	writeFile(t, filepath.Join(d, "Broken", `X\X.toc`), "## X-Curse-Project-ID: 9\n")

	known := map[string]source.Project{
		"curse:334372":   {Source: source.Curse, ID: "334372", Title: "Questie"},
		"curse:1716370":  {Source: source.Curse, ID: "1716370", Title: "ForeverArtisan: WoW Forever Tradeskill Suite"},
		"better-fishing": {Source: source.Curse, ID: "709520", Title: "Better Fishing"},
		"auctionator":    {Source: source.Curse, ID: "6124", Title: "Auctionator"},
		"guidelime":      {Source: source.Curse, ID: "327139", Title: "Guidelime"},
	}
	var calls []string
	lookup := func(ref string) (source.Project, error) {
		calls = append(calls, ref)
		if p, ok := known[ref]; ok {
			return p, nil
		}
		return source.Project{}, fmt.Errorf("slug: %w", source.ErrNotFound)
	}

	folders := scan(t, d)
	st := &State{AddonsDir: d}
	if !st.Adopt(folders, lookup, t.Logf) {
		t.Fatal("Adopt reported no change")
	}
	want := map[string][]string{
		"Questie":        {"Questie", "QuestieDB"},
		"ForeverArtisan": {"ForeverArtisan_Core", "ForeverArtisan_Alchemy"},
		"Better Fishing": {"BetterFishing"},
		"Auctionator":    {"Auctionator"},
		"Guidelime":      {"Guidelime"},
	}
	for name, folders := range want {
		a := st.Find(name)
		if a == nil || !slices.Equal(a.Folders, folders) {
			t.Errorf("%s: got %+v, want folders %v", name, a, folders)
		}
	}
	if a := st.Find("Questie"); a.Version != "12.6.4" || !a.Adopted() {
		t.Errorf("Questie entry %+v", a)
	}
	if len(st.Addons) != len(want) {
		t.Errorf("extra addons tracked: %d", len(st.Addons))
	}
	if !slices.Equal(st.Unresolved, []string{"Journalator", "Journalator_Display"}) {
		t.Errorf("unresolved = %v", st.Unresolved)
	}

	// A second run makes no lookups for folders that are tracked or known to be unresolvable.
	calls = nil
	if st.Adopt(folders, lookup, t.Logf) || len(calls) != 0 {
		t.Fatalf("second Adopt changed state or looked up %v", calls)
	}

	// Deleting a folder by hand drops it from tracking.
	if err := os.RemoveAll(filepath.Join(d, "Guidelime")); err != nil {
		t.Fatal(err)
	}
	st.Adopt(scan(t, d), lookup, t.Logf)
	if st.Find("Guidelime") != nil {
		t.Fatal("Guidelime still tracked after its folder was deleted")
	}
}

func TestPrefixGroupingSkipsInstalledAddons(t *testing.T) {
	d := t.TempDir()
	writeFile(t, filepath.Join(d, "Questie", "Questie.toc"), "## Version: 1\n")
	writeFile(t, filepath.Join(d, "QuestieDB", "QuestieDB.toc"), "## Version: 1\n")
	lookup := func(string) (source.Project, error) {
		return source.Project{}, fmt.Errorf("slug: %w", source.ErrNotFound)
	}
	st := &State{AddonsDir: d, Addons: []*Addon{{Name: "Questie", Source: source.Curse, Project: "1", Release: "9", Folders: []string{"Questie"}}}}
	st.Adopt(scan(t, d), lookup, t.Logf)
	if st.Owner("QuestieDB") != nil || !slices.Equal(st.Unresolved, []string{"QuestieDB"}) {
		t.Fatalf("QuestieDB owner %v, unresolved %v", st.Owner("QuestieDB"), st.Unresolved)
	}
}

func TestSameVersion(t *testing.T) {
	cases := []struct {
		installed, latest string
		want              bool
	}{
		{"12.6.4", "v12.6.4", true},
		{"12.6.4", "v12.9.0", false},
		{"341", "346", false},
		{"34", "346", false},
		{"v1.3.5", "AtlasLoot Classic Forever 1.3.5-101126-16001", true},
		{"1.60.28", "1.60.31-forever", false},
		{"0.9.47", "v0.9.47-forever", true},
		{"", "1", false},
	}
	for _, c := range cases {
		if got := SameVersion(c.installed, source.Release{Version: c.latest}); got != c.want {
			t.Errorf("SameVersion(%q, %q) = %v", c.installed, c.latest, got)
		}
	}
}

func TestNames(t *testing.T) {
	if got := ShortVersion("AtlasLoot Classic Forever", "AtlasLoot Classic Forever 1.3.9-141126-16001"); got != "1.3.9-141126-16001" {
		t.Errorf("ShortVersion = %q", got)
	}
	if got := ShortVersion("Questie", "v12.9.0"); got != "v12.9.0" {
		t.Errorf("ShortVersion = %q", got)
	}
	old := &Addon{Name: "Questie", Source: source.Curse, Project: "1", Folders: []string{"QuestieTrace"}}
	st := &State{Addons: []*Addon{old}}
	p := source.Project{Source: source.GitHub, ID: "Questie/Questie", Title: "Questie"}
	a := st.Add(p, "")
	if a.Name != "Questie (github:Questie/Questie)" {
		t.Fatalf("clashing name = %q", a.Name)
	}
	st.Remove(old)
	st.TidyName(a, p)
	if a.Name != "Questie" {
		t.Fatalf("tidied name = %q", a.Name)
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data", "state.json")
	st, err := Load(path, "/addons")
	if err != nil {
		t.Fatal(err)
	}
	st.Addons = []*Addon{{Name: "B", Source: source.Curse, Project: "2"}, {Name: "a", Source: source.Curse, Project: "1"}}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, "/addons")
	if err != nil || len(got.Addons) != 2 || got.Addons[0].Name != "a" {
		t.Fatalf("reloaded %+v, %v", got, err)
	}
	if _, err := Load(path, "/elsewhere"); err == nil {
		t.Fatal("state for another AddOns directory: want error")
	}
}

func TestLockIsExclusive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	unlock, err := Lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Lock(dir); err == nil || !strings.Contains(err.Error(), "another wam is running") {
		t.Fatalf("second lock: %v", err)
	}
	unlock()
	again, err := Lock(dir)
	if err != nil {
		t.Fatalf("lock after unlock: %v", err)
	}
	again()
}

func TestAdoptRetriesAfterTransientFailures(t *testing.T) {
	d := t.TempDir()
	writeFile(t, filepath.Join(d, "Details", "Details.toc"), "## Version: 1\n")
	writeFile(t, filepath.Join(d, "Questie", "Questie.toc"), "## Version: 1\n## X-Curse-Project-ID: 334372\n")
	writeFile(t, filepath.Join(d, "Unknown", "Unknown.toc"), "## Version: 1\n")

	down := true
	lookup := func(ref string) (source.Project, error) {
		switch {
		case down && ref != "unknown" && ref != "curse:999":
			return source.Project{}, fmt.Errorf("HTTP 503 from %s", ref)
		case ref == "details":
			return source.Project{Source: source.Curse, ID: "1", Title: "Details"}, nil
		case ref == "curse:334372":
			return source.Project{Source: source.Curse, ID: "334372", Title: "Questie"}, nil
		}
		return source.Project{}, fmt.Errorf("slug %s: %w", ref, source.ErrNotFound)
	}
	st := &State{AddonsDir: d}
	st.Adopt(scan(t, d), lookup, t.Logf)
	if slices.Contains(st.Unresolved, "Details") || slices.Contains(st.Unresolved, "Questie") {
		t.Fatalf("folders marked unresolved by a network failure: %v", st.Unresolved)
	}
	if !slices.Contains(st.Unresolved, "Unknown") {
		t.Fatalf("a definitive not-found must still be remembered: %v", st.Unresolved)
	}

	down = false
	st.Adopt(scan(t, d), lookup, t.Logf)
	if st.Find("Details") == nil || st.Find("Questie") == nil {
		t.Fatalf("not adopted once the network was back: %+v", st.Addons)
	}
}
