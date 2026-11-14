// Package install swaps addon releases into the AddOns directory: download, safe unzip,
// backup of what is replaced, and rollback on failure.
package install

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Avicted/wow-addon-manager/internal/source"
	"github.com/Avicted/wow-addon-manager/internal/state"
)

const maxUnpacked = 2 << 30

// ExtractZip unpacks an addon archive into dest and returns the top-level addon folders.
// Backslash separators (Windows-made zips) are normalised, unsafe paths are rejected, and a
// single wrapper directory around the real addon folders is stripped.
func ExtractZip(zipPath, dest string) ([]string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()

	type entry struct {
		f     *zip.File
		parts []string
	}
	var entries []entry
	var total uint64
	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, `\`, "/")
		if strings.HasPrefix(name, "/") || (len(name) > 1 && name[1] == ':') {
			return nil, fmt.Errorf("archive contains absolute path %q", f.Name)
		}
		parts := strings.Split(strings.Trim(name, "/"), "/")
		if slices.Contains(parts, "..") {
			return nil, fmt.Errorf("archive contains unsafe path %q", f.Name)
		}
		parts = slices.DeleteFunc(parts, func(p string) bool { return p == "" || p == "." })
		if len(parts) == 0 || parts[0] == "__MACOSX" || strings.HasSuffix(name, "/") || !f.Mode().IsRegular() {
			continue
		}
		total += f.UncompressedSize64
		entries = append(entries, entry{f, parts})
	}
	if total > maxUnpacked {
		return nil, fmt.Errorf("archive unpacks to %d bytes, refusing", total)
	}

	isTOC := func(p string) bool { return strings.EqualFold(path.Ext(p), ".toc") }
	tops := map[string]bool{}
	tocAtDepth := map[int]bool{}
	for _, e := range entries {
		tops[e.parts[0]] = true
		if isTOC(e.parts[len(e.parts)-1]) {
			tocAtDepth[len(e.parts)] = true
		}
	}
	strip := 0
	if len(tops) == 1 && !tocAtDepth[2] && tocAtDepth[3] {
		strip = 1
	}

	folders := map[string]bool{}
	for _, e := range entries {
		parts := e.parts[strip:]
		if len(parts) < 2 {
			return nil, fmt.Errorf("archive has %q outside any addon folder; not a WoW addon zip?", e.f.Name)
		}
		folders[parts[0]] = true
		if err := extractFile(e.f, filepath.Join(dest, filepath.Join(parts...))); err != nil {
			return nil, err
		}
	}
	if len(folders) == 0 {
		return nil, errors.New("archive is empty")
	}
	out := make([]string, 0, len(folders))
	for f := range folders {
		out = append(out, f)
	}
	sort.Strings(out)
	return out, nil
}

func extractFile(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	w, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, io.LimitReader(rc, int64(f.UncompressedSize64)))
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	return err
}

// move is moveDir; tests replace it to simulate failures.
var move = moveDir

// rename is os.Rename; tests replace it to simulate a cross-device move.
var rename = os.Rename

// moveDir renames, falling back to copy+delete across filesystems.
func moveDir(src, dst string) error {
	err := rename(src, dst)
	if !crossDevice(err) {
		return err
	}
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		_ = os.RemoveAll(dst)
		return err
	}
	return os.RemoveAll(src)
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// Installer changes one AddOns directory. Backups go under DataDir/backups.
type Installer struct {
	AddonsDir, DataDir string
	Log                func(format string, args ...any)
	// Download saves a release file to dst; nil means source.Download. Tests replace it.
	Download func(url, dst string) error
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// BackupsDir is where replaced and removed addons are moved.
func (in *Installer) BackupsDir() string { return filepath.Join(in.DataDir, "backups") }

func (in *Installer) backupDir(name, suffix string) string {
	return filepath.Join(in.BackupsDir(), unsafeName.ReplaceAllString(name, "_"), time.Now().Format("20060102-150405")+suffix)
}

// pruneBackups keeps only the newest backup of an addon.
func pruneBackups(dir string) {
	entries, err := os.ReadDir(filepath.Dir(dir))
	if err != nil {
		return
	}
	for _, e := range entries {
		if p := filepath.Join(filepath.Dir(dir), e.Name()); p != dir {
			_ = os.RemoveAll(p)
		}
	}
}

// verifySHA256 checks a file against a hex digest; an empty digest means there is nothing to check.
func verifySHA256(path, want string) error {
	if want == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, want) {
		return fmt.Errorf("SHA-256 is %s, the release says %s", got, want)
	}
	return nil
}

const stagingPrefix = ".wam-staging-"

// CleanStaging removes staging directories left beside AddOns by a wam that was killed,
// when they are older than olderThan.
func CleanStaging(addonsDir string, olderThan time.Duration) {
	parent := filepath.Dir(addonsDir)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), stagingPrefix) {
			continue
		}
		if fi, err := e.Info(); err == nil && time.Since(fi.ModTime()) > olderThan {
			_ = os.RemoveAll(filepath.Join(parent, e.Name()))
		}
	}
}

// Install downloads rel and swaps its folders into AddOns, backing up what it replaces.
// On failure everything is put back and a is unchanged. With takeover, folders owned by
// another addon are taken from it (an explicit install); otherwise that's an error.
func (in *Installer) Install(st *state.State, a *state.Addon, rel source.Release, takeover bool) error {
	tmp, err := os.MkdirTemp("", "wam-download-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	zipPath := filepath.Join(tmp, "addon.zip")
	download := in.Download
	if download == nil {
		download = source.Download
	}
	if err := download(rel.URL, zipPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}
	if err := verifySHA256(zipPath, rel.SHA256); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	// Stage beside AddOns so the final move is a rename on the same filesystem.
	stage, err := os.MkdirTemp(filepath.Dir(in.AddonsDir), stagingPrefix)
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	folders, err := ExtractZip(zipPath, stage)
	if err != nil {
		return fmt.Errorf("unpack: %w", err)
	}
	for _, f := range folders {
		if o := st.Owner(f); o != nil && o != a && !takeover {
			return fmt.Errorf("folder %s belongs to %s; to switch it to %s run: wam install %s", f, o.Name, a.Name, a.Ref())
		}
	}

	backup := in.backupDir(a.Name, "")
	var movedOld, movedNew []string
	rollback := func(cause error) error {
		for _, f := range movedNew {
			_ = os.RemoveAll(filepath.Join(in.AddonsDir, f))
		}
		for _, f := range movedOld {
			if err := move(filepath.Join(backup, f), filepath.Join(in.AddonsDir, f)); err != nil {
				return fmt.Errorf("%w; restoring %s from %s also failed: %w", cause, f, backup, err)
			}
		}
		return cause
	}
	for _, f := range folders {
		live := filepath.Join(in.AddonsDir, f)
		if !exists(live) {
			continue
		}
		if err := os.MkdirAll(backup, 0o755); err != nil {
			return rollback(err)
		}
		if err := move(live, filepath.Join(backup, f)); err != nil {
			return rollback(err)
		}
		movedOld = append(movedOld, f)
	}
	for _, f := range folders {
		if err := move(filepath.Join(stage, f), filepath.Join(in.AddonsDir, f)); err != nil {
			return rollback(err)
		}
		movedNew = append(movedNew, f)
	}
	if len(movedOld) > 0 {
		pruneBackups(backup)
	}

	for _, f := range a.Folders {
		if !slices.Contains(folders, f) && exists(filepath.Join(in.AddonsDir, f)) {
			in.Log("  note: %s is no longer part of %s; left in place", f, a.Name)
		}
	}
	for _, f := range folders {
		if o := st.Owner(f); o != nil && o != a {
			in.Log("  %s now belongs to %s (was %s)", f, a.Name, o.Name)
			st.Disown(f)
		}
		st.Resolved(f)
	}
	a.Folders, a.Release, a.Version = folders, rel.ID, rel.Version
	return nil
}

// Remove moves folders out of AddOns into a backup and returns where they went.
func (in *Installer) Remove(name string, folders []string) (string, error) {
	backup := in.backupDir(name, "-removed")
	if err := os.MkdirAll(backup, 0o755); err != nil {
		return "", err
	}
	for _, f := range folders {
		live := filepath.Join(in.AddonsDir, f)
		if !exists(live) {
			continue
		}
		if err := move(live, filepath.Join(backup, f)); err != nil {
			return backup, err
		}
	}
	pruneBackups(backup)
	return backup, nil
}
