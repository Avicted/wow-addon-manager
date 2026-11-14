// Package state records which project each AddOns folder came from and which release is
// installed, and adopts folders that were installed without wam.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Avicted/wow-addon-manager/internal/source"
	"github.com/Avicted/wow-addon-manager/internal/toc"
)

// Addon is one tracked project and the AddOns folders it owns.
type Addon struct {
	Name    string   `json:"name"`
	Source  string   `json:"source"`
	Project string   `json:"project"`
	Release string   `json:"release,omitempty"` // CurseForge file id or GitHub tag; empty when adopted
	Version string   `json:"version,omitempty"`
	Folders []string `json:"folders"`
}

// Ref is the reference wam install accepts for this addon.
func (a *Addon) Ref() string { return a.Source + ":" + a.Project }

// Origin is the source project this addon is installed from.
func (a *Addon) Origin() source.Project { return source.Project{Source: a.Source, ID: a.Project} }

// Adopted addons were installed outside wam, so the exact installed release is unknown.
func (a *Addon) Adopted() bool { return a.Release == "" }

// State is everything wam knows about one AddOns directory.
type State struct {
	AddonsDir  string   `json:"addons_dir"`
	Addons     []*Addon `json:"addons"`
	Unresolved []string `json:"unresolved,omitempty"` // folders whose source lookup already failed
	path       string
}

// Load reads the state file, or starts empty if there is none. A state file belongs to one
// AddOns directory; using it for another is an error.
func Load(path, addonsDir string) (*State, error) {
	st := &State{AddonsDir: addonsDir, path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, st); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if st.AddonsDir != addonsDir {
		return nil, fmt.Errorf("%s tracks %s, not %s; set WAM_DATA_DIR to use a separate state for that directory", path, st.AddonsDir, addonsDir)
	}
	return st, nil
}

// Save writes the state atomically.
func (s *State) Save() error {
	sort.Slice(s.Addons, func(i, j int) bool { return strings.ToLower(s.Addons[i].Name) < strings.ToLower(s.Addons[j].Name) })
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Find matches an addon by name or by one of its folders, case-insensitively.
func (s *State) Find(name string) *Addon {
	for _, a := range s.Addons {
		if strings.EqualFold(a.Name, name) {
			return a
		}
	}
	for _, a := range s.Addons {
		for _, f := range a.Folders {
			if strings.EqualFold(f, name) {
				return a
			}
		}
	}
	return nil
}

// ByProject returns the addon installed from p.
func (s *State) ByProject(p source.Project) *Addon {
	for _, a := range s.Addons {
		if a.Source == p.Source && a.Project == p.ID {
			return a
		}
	}
	return nil
}

// Owner returns the addon owning a folder.
func (s *State) Owner(folder string) *Addon {
	for _, a := range s.Addons {
		if slices.Contains(a.Folders, folder) {
			return a
		}
	}
	return nil
}

// Remove forgets an addon.
func (s *State) Remove(a *Addon) {
	s.Addons = slices.DeleteFunc(s.Addons, func(x *Addon) bool { return x == a })
}

// Disown drops a folder from whichever addon owns it, and the addon if it owned nothing else.
func (s *State) Disown(folder string) {
	if a := s.Owner(folder); a != nil {
		a.Folders = slices.DeleteFunc(a.Folders, func(f string) bool { return f == folder })
		if len(a.Folders) == 0 {
			s.Remove(a)
		}
	}
	s.Resolved(folder)
}

// Resolved removes a folder from the unresolved list.
func (s *State) Resolved(folder string) {
	s.Unresolved = slices.DeleteFunc(s.Unresolved, func(f string) bool { return f == folder })
}

// Add returns the entry for p, creating it with a name that doesn't clash with another entry.
func (s *State) Add(p source.Project, fallbackName string) *Addon {
	if a := s.ByProject(p); a != nil {
		return a
	}
	name := ShortName(p, fallbackName)
	if other := s.Find(name); other != nil {
		name = fmt.Sprintf("%s (%s)", name, p.Ref())
	}
	a := &Addon{Name: name, Source: p.Source, Project: p.ID}
	s.Addons = append(s.Addons, a)
	return a
}

// ShortName turns "ForeverArtisan: WoW Forever Tradeskill Suite, ..." into "ForeverArtisan".
func ShortName(p source.Project, fallback string) string {
	name, _, _ := strings.Cut(p.Title, ": ")
	name, _, _ = strings.Cut(name, " - ")
	if name = strings.TrimSpace(name); name == "" {
		return fallback
	}
	return name
}

// TidyName drops the disambiguating "(source:id)" suffix once nothing else uses the plain name.
func (s *State) TidyName(a *Addon, p source.Project) {
	name := ShortName(p, a.Name)
	if o := s.Find(name); o == nil || o == a {
		a.Name = name
	}
}

// ShortVersion drops a leading addon name from a version
// ("AtlasLoot Classic Forever 1.3.9-141126" -> "1.3.9-141126").
func ShortVersion(name, v string) string {
	if len(v) > len(name) && strings.EqualFold(v[:len(name)], name) {
		if rest := strings.TrimLeft(v[len(name):], " -_"); rest != "" {
			return rest
		}
	}
	return v
}

// Lookup resolves a project reference (source.Resolve; replaced in tests).
type Lookup func(ref string) (source.Project, error)

// Adopt brings folders that wam didn't install under tracking where their source can be
// found, and forgets folders that no longer exist. It reports whether the state changed.
func (s *State) Adopt(folders []toc.Folder, lookup Lookup, log func(string, ...any)) bool {
	present := map[string]bool{}
	for _, f := range folders {
		present[f.Name] = true
	}
	changed := false
	for _, a := range slices.Clone(s.Addons) {
		for _, f := range a.Folders {
			if !present[f] {
				s.Disown(f)
				changed = true
			}
		}
	}
	s.Unresolved = slices.DeleteFunc(s.Unresolved, func(f string) bool { return !present[f] })

	var todo []toc.Folder
	for _, f := range folders {
		if !f.Broken && s.Owner(f.Name) == nil {
			todo = append(todo, f)
		}
	}
	attach := func(a *Addon, f toc.Folder) {
		a.Folders = append(a.Folders, f.Name)
		if a.Adopted() && a.Version == "" {
			a.Version = f.Get("Version")
		}
		s.Resolved(f.Name)
		changed = true
	}

	// Pass 1: folders that name their CurseForge project. Pass 2: everything else, first by
	// folder-name prefix (ForeverArtisan_*, QuestieDB) and only then by guessing a slug.
	var rest []toc.Folder
	for _, f := range todo {
		id := f.Get("X-Curse-Project-ID")
		if !source.IsProjectID(id) || slices.Contains(s.Unresolved, f.Name) {
			rest = append(rest, f)
			continue
		}
		p, err := lookup("curse:" + id)
		if err != nil {
			log("%s: CurseForge project %s: %v", f.Name, id, err)
			rest = append(rest, f)
			continue
		}
		attach(s.Add(p, f.Name), f)
	}
	sort.SliceStable(rest, func(i, j int) bool { return len(rest[i].Name) < len(rest[j].Name) })
	for _, f := range rest {
		if a := s.prefixOwner(f.Name); a != nil {
			attach(a, f)
			continue
		}
		if slices.Contains(s.Unresolved, f.Name) {
			continue
		}
		if p, ok := identify(f, lookup); ok {
			attach(s.Add(p, f.Name), f)
			continue
		}
		s.Unresolved = append(s.Unresolved, f.Name)
		changed = true
	}
	return changed
}

// RootName is a folder name up to its first underscore: ForeverArtisan_Core -> ForeverArtisan.
func RootName(folder string) string {
	root, _, _ := strings.Cut(folder, "_")
	return root
}

// prefixOwner finds the adopted addon owning a folder whose root name prefixes this one.
// Addons wam installed are skipped: their folder set is exactly what their zip contained.
func (s *State) prefixOwner(folder string) *Addon {
	var best *Addon
	bestLen := 0
	for _, a := range s.Addons {
		if !a.Adopted() {
			continue
		}
		for _, f := range a.Folders {
			root := RootName(f)
			if len(root) >= 4 && len(root) > bestLen && len(folder) > len(root) && strings.HasPrefix(folder, root) {
				best, bestLen = a, len(root)
			}
		}
	}
	return best
}

var (
	nonAlnum   = regexp.MustCompile(`[^a-z0-9]+`)
	camelSplit = regexp.MustCompile(`([a-z0-9])([A-Z])`)
)

// Normalize lower-cases s and drops everything but letters and digits.
func Normalize(s string) string { return nonAlnum.ReplaceAllString(strings.ToLower(s), "") }

// Slug turns free text into a CurseForge-style slug ("Better Fishing" -> "better-fishing").
func Slug(s string) string {
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// identify finds a folder's CurseForge project from its X-Website, or by guessing the slug from
// its name and accepting the guess only when the project title matches.
func identify(f toc.Folder, lookup Lookup) (source.Project, bool) {
	if slug, ok := source.CurseSlug(f.Get("X-Website")); ok {
		if p, err := lookup(slug); err == nil {
			return p, true
		}
	}
	title := toc.CleanTitle(f.Get("Title"))
	var guesses []string
	for _, g := range []string{
		strings.ToLower(strings.ReplaceAll(f.Name, "_", "-")),
		strings.ToLower(camelSplit.ReplaceAllString(strings.ReplaceAll(f.Name, "_", "-"), "$1-$2")),
		Slug(title),
	} {
		if g != "" && !slices.Contains(guesses, g) {
			guesses = append(guesses, g)
		}
	}
	for _, g := range guesses {
		p, err := lookup(g)
		if err != nil {
			continue
		}
		if n := Normalize(p.Title); n == Normalize(title) || n == Normalize(f.Name) {
			return p, true
		}
	}
	return source.Project{}, false
}

var versionToken = regexp.MustCompile(`[^A-Za-z0-9.]+`)

// SameVersion guesses whether an adopted addon's TOC version is the given release, by looking
// for it as a whole token of the release name ("12.6.4" in "Questie-v12.6.4").
func SameVersion(installed string, rel source.Release) bool {
	want := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(installed)), "v")
	if want == "" {
		return false
	}
	for _, tok := range versionToken.Split(strings.ToLower(rel.Version), -1) {
		if strings.TrimPrefix(tok, "v") == want {
			return true
		}
	}
	return false
}
