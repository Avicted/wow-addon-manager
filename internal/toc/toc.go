// Package toc reads the AddOns directory: one Folder per addon directory, with the header
// fields of the .toc file the Forever client would load.
package toc

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Only the header of a .toc is read; some (QuestieDB) are tens of megabytes of data.
const readLimit = 16 << 10

// Folder is one directory in Interface/AddOns.
type Folder struct {
	Name   string
	Fields map[string]string // lower-cased "## Key" -> value
	Broken bool              // files with literal backslashes: a Windows zip unpacked on Linux
}

// Get returns a TOC field by name, case-insensitively.
func (f Folder) Get(key string) string { return f.Fields[strings.ToLower(key)] }

// Parse reads "## Key: Value" lines from the start of a .toc file. The first value of a
// repeated key wins.
func Parse(r io.Reader) map[string]string {
	fields := map[string]string{}
	sc := bufio.NewScanner(io.LimitReader(r, readLimit))
	sc.Buffer(make([]byte, 0, 4096), readLimit+1)
	first := true
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if first {
			line = strings.TrimPrefix(line, "\ufeff")
			first = false
		}
		if !strings.HasPrefix(line, "##") {
			continue
		}
		k, v, ok := strings.Cut(line[2:], ":")
		k = strings.ToLower(strings.TrimSpace(k))
		if !ok || k == "" {
			continue
		}
		if _, dup := fields[k]; !dup {
			fields[k] = strings.TrimSpace(v)
		}
	}
	return fields
}

// rank orders the .toc files of a folder by how likely the Forever client loads them.
func rank(folder, file string) int {
	base := strings.TrimSuffix(file, filepath.Ext(file))
	if len(base) < len(folder) || !strings.EqualFold(base[:len(folder)], folder) {
		return 10
	}
	switch strings.ToLower(base[len(folder):]) {
	case "_forever", "-forever":
		return 0
	case "_camelot", "-camelot":
		return 1
	case "":
		return 2
	}
	return 3
}

// Read loads one addon folder.
func Read(addonsDir, name string) (Folder, error) {
	f := Folder{Name: name, Fields: map[string]string{}}
	entries, err := os.ReadDir(filepath.Join(addonsDir, name))
	if err != nil {
		return f, err
	}
	var tocs []string
	for _, e := range entries {
		if strings.Contains(e.Name(), `\`) {
			f.Broken = true
		}
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".toc") {
			tocs = append(tocs, e.Name())
		}
	}
	sort.SliceStable(tocs, func(i, j int) bool { return rank(name, tocs[i]) < rank(name, tocs[j]) })
	for i, t := range tocs {
		fields, err := readFile(filepath.Join(addonsDir, name, t))
		if err != nil {
			return f, err
		}
		if i == 0 {
			f.Fields = fields
		}
		// In a broken folder every .toc is a flattened "Sub\Sub.toc"; find the one naming the project.
		if !f.Broken || fields["x-curse-project-id"] != "" {
			if f.Broken {
				f.Fields = fields
			}
			break
		}
	}
	return f, nil
}

func readFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return Parse(file), nil
}

// Scan reads every addon folder, skipping hidden entries (wam's own staging directories).
func Scan(addonsDir string) ([]Folder, error) {
	entries, err := os.ReadDir(addonsDir)
	if err != nil {
		return nil, err
	}
	var out []Folder
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if fi, err := os.Stat(filepath.Join(addonsDir, e.Name())); err != nil || !fi.IsDir() {
			continue
		}
		f, err := Read(addonsDir, e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

var uiEscapes = regexp.MustCompile(`\|c[0-9a-fA-F]{8}|\|r|\|T[^|]*\|t|\|A[^|]*\|a`)

// CleanTitle removes WoW colour and texture escapes from a TOC title.
func CleanTitle(s string) string {
	return strings.Join(strings.Fields(uiEscapes.ReplaceAllString(s, "")), " ")
}
