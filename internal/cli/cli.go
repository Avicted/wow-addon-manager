// Package cli implements the wam commands.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/Avicted/wow-addon-manager/internal/install"
	"github.com/Avicted/wow-addon-manager/internal/source"
	"github.com/Avicted/wow-addon-manager/internal/state"
	"github.com/Avicted/wow-addon-manager/internal/toc"
)

// defaultAddonsDir is where a Battle.net-under-Wine install keeps WoW Forever's addons.
const defaultAddonsDir = "Games/battlenet/drive_c/Program Files (x86)/World of Warcraft/_forever_/Interface/AddOns"

const usage = `wam - WoW Forever addon manager

Usage:
  wam [flags] [command]

Commands:
  status                  show installed addons and available updates (default)
  update [-n] [name...]   update all addons, or only the named ones (-n: dry run)
  install <ref>           install an addon; <ref> is a CurseForge URL, slug or
                          project id (curse:1422985), or a GitHub repo URL
  track <folder> <ref>    record where an untracked folder comes from
  remove <name|folder>    move an addon's folders to the backup directory
  search <text>           find addons with Forever builds
  version                 print the version

Flags:
  -addons-dir DIR   AddOns directory (env WAM_ADDONS_DIR,
                    default ~/` + defaultAddonsDir + `)
  -rescan           retry identifying folders that couldn't be matched before

Environment:
  WAM_DATA_DIR      state and backups (default ~/.local/share/wam)
  GITHUB_TOKEN      raises GitHub's API rate limit
`

type env struct {
	addonsDir string
	out, err  io.Writer
	inst      *install.Installer
	lookup    state.Lookup
	latest    func(source.Project) (source.Release, error)
	color     bool
	ctx       context.Context // cancelled by Ctrl-C or SIGTERM; nil means never
}

// interrupted reports whether the user asked wam to stop.
func (e *env) interrupted() bool { return e.ctx != nil && e.ctx.Err() != nil }

// paint wraps s in an ANSI color when the output is a terminal.
func (e *env) paint(code, s string) string {
	if !e.color {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// isTerminal reports whether w is a terminal and NO_COLOR is unset.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok || os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func (e *env) printf(format string, args ...any) { _, _ = fmt.Fprintf(e.out, format+"\n", args...) }
func (e *env) logf(format string, args ...any)   { _, _ = fmt.Fprintf(e.err, format+"\n", args...) }

// Run executes one wam command and returns the process exit code.
func Run(args []string, out, errOut io.Writer, version string) int {
	source.UserAgent = "wam/" + version + " (WoW Forever addon manager)"
	home, _ := os.UserHomeDir()

	fl := flag.NewFlagSet("wam", flag.ContinueOnError)
	fl.SetOutput(errOut)
	fl.Usage = func() { _, _ = fmt.Fprint(errOut, usage) }
	addonsDir := fl.String("addons-dir", envOr("WAM_ADDONS_DIR", filepath.Join(home, defaultAddonsDir)), "")
	rescan := fl.Bool("rescan", false, "")
	showVersion := fl.Bool("version", false, "")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	rest := fl.Args()
	cmd := "status"
	if len(rest) > 0 {
		cmd, rest = rest[0], rest[1:]
	}
	switch {
	case *showVersion || cmd == "version":
		_, _ = fmt.Fprintln(out, "wam", version)
		return 0
	case cmd == "help":
		_, _ = fmt.Fprint(out, usage)
		return 0
	}

	dir, err := filepath.Abs(*addonsDir)
	if err == nil {
		_, err = os.Stat(dir)
	}
	dataDir := dataDir(home)
	// The first Ctrl-C lets the addon being installed finish, so AddOns is never left half
	// swapped; a second one, with the default handling restored, ends wam at once.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() { <-ctx.Done(); stop() }()
	e := &env{ctx: ctx, addonsDir: dir, out: out, err: errOut, lookup: source.Resolve, latest: source.Latest, color: isTerminal(out)}
	e.inst = &install.Installer{AddonsDir: dir, DataDir: dataDir, Log: e.logf}
	if err != nil {
		e.logf("wam: AddOns directory: %v", err)
		return 1
	}
	if cmd == "update" || cmd == "install" {
		install.CleanStaging(dir, time.Hour)
	}
	unlock, err := state.Lock(dataDir)
	if err != nil {
		e.logf("wam: %v", err)
		return 1
	}
	defer unlock()
	st, err := state.Load(filepath.Join(dataDir, "state.json"), dir)
	if err != nil {
		e.logf("wam: %v", err)
		return 1
	}
	if *rescan {
		st.Unresolved = nil
	}

	cmds := map[string]func(*env, *state.State, []string) error{
		"status": cmdStatus, "update": cmdUpdate, "install": cmdInstall,
		"track": cmdTrack, "remove": cmdRemove, "search": cmdSearch,
	}
	fn, ok := cmds[cmd]
	if !ok {
		e.logf("wam: unknown command %q\n", cmd)
		_, _ = fmt.Fprint(errOut, usage)
		return 2
	}
	if err := fn(e, st, rest); err != nil {
		e.logf("wam: %v", err)
		return 1
	}
	return 0
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func dataDir(home string) string {
	if d := os.Getenv("WAM_DATA_DIR"); d != "" {
		return d
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "wam")
	}
	return filepath.Join(home, ".local", "share", "wam")
}

// sync scans AddOns, adopts new folders and saves the state if anything changed.
func (e *env) sync(st *state.State) ([]toc.Folder, error) {
	folders, err := toc.Scan(e.addonsDir)
	if err != nil {
		return nil, err
	}
	for _, f := range folders {
		if f.Err != nil {
			e.logf("warning: can't read %s: %v", f.Name, f.Err)
		}
	}
	if st.Adopt(folders, e.lookup, e.logf) {
		if err := st.Save(); err != nil {
			return nil, err
		}
	}
	return folders, nil
}

type check struct {
	rel source.Release
	err error
}

func (c check) outdated(a *state.Addon) bool {
	if c.err != nil {
		return false
	}
	if a.Adopted() {
		return !state.SameVersion(a.Version, c.rel)
	}
	return c.rel.ID != a.Release
}

func (e *env) checkAll(addons []*state.Addon) map[*state.Addon]check {
	res := make(map[*state.Addon]check, len(addons))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for _, a := range addons {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			rel, err := e.latest(a.Origin())
			mu.Lock()
			res[a] = check{rel, err}
			mu.Unlock()
		})
	}
	wg.Wait()
	return res
}

func cmdStatus(e *env, st *state.State, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("status takes no arguments")
	}
	folders, err := e.sync(st)
	if err != nil {
		return err
	}
	checks := e.checkAll(st.Addons)
	tw := tabwriter.NewWriter(e.out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ADDON\tINSTALLED\tLATEST\tSOURCE\tSTATUS")
	updates, guessed := 0, false
	var errs [][2]string
	for _, a := range st.Addons {
		c := checks[a]
		status := e.paint("32", "ok")
		switch {
		case c.err != nil:
			status = e.paint("31", "error")
			errs = append(errs, [2]string{a.Name, c.err.Error()})
		case c.outdated(a):
			status = e.paint("33", "update")
			updates++
		}
		installed := state.ShortVersion(a.Name, a.Version)
		if a.Adopted() && c.err == nil {
			installed += " ~"
			guessed = true
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", a.Name, installed, state.ShortVersion(a.Name, c.rel.Version), a.Ref(), status)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	var broken [][2]string
	untracked := map[string][]string{}
	for _, f := range folders {
		switch {
		case f.Err != nil:
			continue
		case f.Broken:
			hint := ""
			if id := f.Get("X-Curse-Project-ID"); id != "" {
				hint = fmt.Sprintf("fix: wam install curse:%s && wam remove %q", id, f.Name)
			}
			broken = append(broken, [2]string{f.Name, hint})
		case st.Owner(f.Name) == nil:
			root := state.RootName(f.Name)
			untracked[root] = append(untracked[root], f.Name)
		}
	}
	roots := make([]string, 0, len(untracked))
	for r := range untracked {
		roots = append(roots, r)
	}
	sort.Strings(roots)
	var untrackedRows [][2]string
	for _, r := range roots {
		names := untracked[r]
		label := names[0]
		if len(names) > 1 {
			label = fmt.Sprintf("%s (+%d folders)", r, len(names)-1)
		}
		untrackedRows = append(untrackedRows, [2]string{label, fmt.Sprintf("wam track %s <curseforge-url>", names[0])})
	}

	e.section(e.paint("31", "Errors:"), errs)
	e.section("Not tracked (wam doesn't know where these come from):", untrackedRows)
	e.section(e.paint("31", "Broken (unpacked with literal backslashes; the game can't load them):"), broken)

	e.printf("")
	if guessed {
		e.printf("~ installed outside wam: compared by .toc version until the first update.")
	}
	if updates > 0 {
		e.printf("%s available. Run: wam update", plural(updates, "update"))
	} else {
		e.printf("All %s up to date.", plural(len(st.Addons), "addon"))
	}
	return nil
}

// section prints a titled, aligned two-column list after a blank line; nothing if rows is empty.
func (e *env) section(title string, rows [][2]string) {
	if len(rows) == 0 {
		return
	}
	e.printf("\n%s", title)
	tw := tabwriter.NewWriter(e.out, 0, 0, 2, ' ', 0)
	for _, r := range rows {
		_, _ = fmt.Fprintf(tw, "  %s\t%s\n", r[0], r[1])
	}
	_ = tw.Flush()
}

func cmdUpdate(e *env, st *state.State, args []string) error {
	dry := false
	var names []string
	for _, a := range args {
		if a == "-n" || a == "--dry-run" || a == "-dry-run" {
			dry = true
		} else {
			names = append(names, a)
		}
	}
	if _, err := e.sync(st); err != nil {
		return err
	}
	targets := st.Addons
	if len(names) > 0 {
		targets = nil
		for _, n := range names {
			a := st.Find(n)
			if a == nil {
				return fmt.Errorf("%s is not a tracked addon (see wam status)", n)
			}
			if !slices.Contains(targets, a) {
				targets = append(targets, a)
			}
		}
	}
	checks := e.checkAll(targets)
	var todo []*state.Addon
	failed := 0
	for _, a := range targets {
		c := checks[a]
		switch {
		case c.err != nil:
			e.logf("%s: %v", a.Name, c.err)
			failed++
		case c.outdated(a):
			todo = append(todo, a)
		}
	}
	if len(todo) == 0 {
		e.printf("Everything is up to date.")
		return failedErr(failed)
	}
	if !dry && install.WowRunning() {
		e.logf("warning: WoW is running; updated addons load after /reload or a restart")
	}
	tw := tabwriter.NewWriter(e.out, 0, 0, 2, ' ', 0)
	updated, skipped := 0, 0
	for i, a := range todo {
		if !dry && e.interrupted() {
			skipped = len(todo) - i
			break
		}
		rel := checks[a].rel
		status := e.paint("33", "would update")
		if !dry {
			if err := e.inst.Install(st, a, rel, false); err != nil {
				status = e.paint("31", "failed: "+err.Error())
				failed++
			} else if err := st.Save(); err != nil {
				_ = tw.Flush()
				return err
			} else {
				status = e.paint("32", "updated")
				updated++
			}
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t->\t%s\t%s\n", a.Name, orDash(state.ShortVersion(a.Name, a.Version)), state.ShortVersion(a.Name, rel.Version), status)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if dry {
		e.printf("\nDry run: %s, nothing changed.", plural(len(todo), "update"))
	} else {
		msg := "Updated " + plural(updated, "addon")
		if failed > 0 {
			msg += fmt.Sprintf(", %d failed", failed)
		}
		if skipped > 0 {
			msg += fmt.Sprintf(", %d skipped", skipped)
		}
		e.printf("\n%s.", msg)
	}
	if skipped > 0 {
		return fmt.Errorf("interrupted: %s not updated", plural(skipped, "addon"))
	}
	return failedErr(failed)
}

func failedErr(n int) error {
	if n > 0 {
		return fmt.Errorf("%d addon(s) failed", n)
	}
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func cmdInstall(e *env, st *state.State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: wam install <curseforge-url|slug|curse:id|github-url>")
	}
	if _, err := e.sync(st); err != nil {
		return err
	}
	p, err := e.lookup(args[0])
	if err != nil {
		return err
	}
	a := st.ByProject(p)
	if a != nil && !a.Adopted() {
		return fmt.Errorf("%s is already installed; use: wam update %q", a.Name, a.Name)
	}
	rel, err := e.latest(p)
	if err != nil {
		return err
	}
	isNew := a == nil
	if isNew {
		a = st.Add(p, p.ID)
	}
	e.printf("Installing %s %s", a.Name, state.ShortVersion(a.Name, rel.Version))
	if install.WowRunning() {
		e.logf("warning: WoW is running; the addon loads after a restart")
	}
	if err := e.inst.Install(st, a, rel, true); err != nil {
		if isNew {
			st.Remove(a)
		}
		return err
	}
	st.TidyName(a, p)
	e.printf("Installed folders: %s", strings.Join(a.Folders, ", "))
	return st.Save()
}

func cmdTrack(e *env, st *state.State, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: wam track <folder> <curseforge-url|slug|curse:id|github-url>")
	}
	folder := filepath.Base(args[0])
	f, err := toc.Read(e.addonsDir, folder)
	if err != nil {
		return err
	}
	p, err := e.lookup(args[1])
	if err != nil {
		return err
	}
	st.Disown(folder)
	a := st.Add(p, folder)
	a.Folders = append(a.Folders, folder)
	if a.Adopted() && a.Version == "" {
		a.Version = f.Get("Version")
	}
	e.printf("%s is now tracked as %s (%s)", folder, a.Name, a.Ref())
	return st.Save()
}

func cmdRemove(e *env, st *state.State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: wam remove <addon|folder>")
	}
	if _, err := e.sync(st); err != nil {
		return err
	}
	name, folders := args[0], []string{filepath.Base(args[0])}
	a := st.Find(args[0])
	if a != nil {
		name, folders = a.Name, a.Folders
	} else if _, err := os.Lstat(filepath.Join(e.addonsDir, folders[0])); err != nil {
		return fmt.Errorf("no addon or folder named %s", args[0])
	}
	backup, err := e.inst.Remove(name, folders)
	if err != nil {
		return err
	}
	if a != nil {
		st.Remove(a)
	} else {
		st.Disown(folders[0])
	}
	e.printf("Removed %s; moved %s to %s", name, strings.Join(folders, ", "), backup)
	return st.Save()
}

func cmdSearch(e *env, st *state.State, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: wam search <text>")
	}
	text := strings.Join(args, " ")
	q := state.Normalize(text)
	// The query itself as a CurseForge slug ("details"), plus Wago's list of addons with Forever
	// builds, whose slugs often differ from CurseForge's.
	var exact source.Project
	var wg sync.WaitGroup
	wg.Go(func() { exact, _ = source.CurseBySlug(state.Slug(text)) })
	slugs, err := source.WagoForeverSlugs()
	if err != nil {
		e.logf("Wago addon list unavailable: %v", err)
	}
	var hits []source.WagoSlug
	for _, s := range slugs {
		if strings.Contains(state.Normalize(s.Slug), q) {
			hits = append(hits, s)
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Downloads > hits[j].Downloads })
	if len(hits) > 10 {
		hits = hits[:10]
	}
	projects := make([]source.Project, len(hits))
	for i, h := range hits {
		wg.Go(func() { projects[i], _ = source.CurseBySlug(h.Slug) })
	}
	wg.Wait()

	installWith := func(p source.Project) string {
		if st.ByProject(p) != nil {
			return "installed"
		}
		return "wam install " + p.Ref()
	}
	tw := tabwriter.NewWriter(e.out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tWAGO DOWNLOADS\tINSTALL WITH")
	if exact.ID != "" {
		_, _ = fmt.Fprintf(tw, "%s\t\t%s\n", exact.Title, installWith(exact))
	}
	for i, h := range hits {
		if p := projects[i]; p.ID != "" {
			if p.ID != exact.ID {
				_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\n", p.Title, h.Downloads, installWith(p))
			}
			continue
		}
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\n", h.Slug, h.Downloads, "not matched on CurseForge; see https://addons.wago.io/addons/"+h.Slug)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if exact.ID == "" && len(hits) == 0 {
		e.printf("Nothing found for %q. wam install also takes a CurseForge or GitHub URL.", text)
	} else {
		e.printf("\nCurseForge results may lack a Forever build; wam install says so if they do.")
	}
	return nil
}
