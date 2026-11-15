// Package cli implements the wam commands.
package cli

import (
	"bufio"
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

var usage = `wam - WoW Forever addon manager

Usage:
  wam [flags] [command]

Commands:
  status                  show installed addons and available updates (default)
  update [-n] [name...]   update all addons, or only the named ones (-n: dry run)
  install <ref>           install an addon; <ref> is a CurseForge URL, slug or
                          project id (curse:1422985), or a GitHub repo URL
  track <folder> <ref>    record where an untracked folder comes from
  remove <name|folder>    remove an addon, keeping a zip backup in AddOns-backups
  search <text>           find addons with Forever builds
  version                 print the version

Flags:
  -addons-dir DIR   AddOns directory (env WAM_ADDONS_DIR, default
                    ` + addonsDirHint + `)
  -rescan           retry identifying folders that couldn't be matched before

Environment:
  WAM_DATA_DIR      state (default ` + dataDirHint + `)
  GITHUB_TOKEN      raises GitHub's API rate limit
`

type env struct {
	addonsDir string
	dataDir   string // where state.json lives; empty in tests that don't need it
	out, err  io.Writer
	inst      *install.Installer
	lookup    state.Lookup
	latest    func(source.Project) (source.Release, error)
	bySlug    func(slug string) (source.Project, error)
	forever   func() ([]source.WagoSlug, error)
	color     bool
	ctx       context.Context // cancelled by Ctrl-C, SIGTERM or "cancel" on stdin with -json; nil means never

	json bool       // -json: events on stdout instead of text (json.go)
	data any        // what the command reports; the result event carries it with -json
	mu   sync.Mutex // serialises emit
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

// Run executes one wam command and returns the process exit code. With -json, a line
// "cancel" on in stops the command the way Ctrl-C does.
func Run(args []string, in io.Reader, out, errOut io.Writer, version string) int {
	source.UserAgent = "wam/" + version + " (WoW Forever addon manager)"
	home, _ := os.UserHomeDir()

	fl := flag.NewFlagSet("wam", flag.ContinueOnError)
	fl.SetOutput(errOut)
	fl.Usage = func() { _, _ = fmt.Fprint(errOut, usage) }
	addonsDir := fl.String("addons-dir", envOr("WAM_ADDONS_DIR", defaultAddonsDir(home)), "")
	rescan := fl.Bool("rescan", false, "")
	showVersion := fl.Bool("version", false, "")
	jsonOut := fl.Bool("json", false, "")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	rest := fl.Args()
	cmd := "status"
	if len(rest) > 0 {
		cmd, rest = rest[0], rest[1:]
	}
	e := &env{out: out, err: errOut, json: *jsonOut, lookup: source.Resolve, latest: source.Latest,
		bySlug: source.CurseBySlug, forever: source.WagoForeverSlugs, color: !*jsonOut && isTerminal(out)}
	switch {
	case *showVersion || cmd == "version":
		if e.json {
			e.emit(resultEvent{Event: "result", OK: true, Data: map[string]string{"version": version}})
		} else {
			_, _ = fmt.Fprintln(out, "wam", version)
		}
		return 0
	case cmd == "help":
		_, _ = fmt.Fprint(out, usage)
		return 0
	}

	// The first Ctrl-C lets the addon being installed finish, so AddOns is never left half
	// swapped; a second one, with the default handling restored, ends wam at once.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { <-ctx.Done(); stop() }()
	if e.json && in != nil {
		go watchCancel(in, cancel)
	}
	e.ctx = ctx
	code, err := e.run(cmd, rest, home, *addonsDir, *rescan)
	switch {
	case e.json:
		e.emit(resultEvent{Event: "result", OK: err == nil, Data: e.data, Error: describe(err)})
	case err != nil:
		e.logf("wam: %v", err)
		if code == 2 {
			_, _ = fmt.Fprint(errOut, usage)
		}
	}
	return code
}

// watchCancel calls cancel when the line "cancel" arrives on in. The GUI can't send Ctrl-C
// to a child process on Windows, so it writes that instead. End of input changes nothing.
func watchCancel(in io.Reader, cancel func()) {
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "cancel" {
			cancel()
		}
	}
}

// run sets up the AddOns directory, the lock and the state, then runs cmd.
func (e *env) run(cmd string, args []string, home, addonsDir string, rescan bool) (int, error) {
	dir, err := filepath.Abs(addonsDir)
	if err == nil {
		_, err = os.Stat(dir)
	}
	dataDir := dataDir(home)
	e.addonsDir = dir
	e.dataDir = dataDir
	e.inst = &install.Installer{AddonsDir: dir, Log: e.logf}
	if err != nil {
		return 1, fmt.Errorf("%w: %w", errAddonsDir, err)
	}
	cmds := map[string]func(*env, *state.State, []string) error{
		"status": cmdStatus, "update": cmdUpdate, "install": cmdInstall,
		"track": cmdTrack, "remove": cmdRemove, "search": cmdSearch,
	}
	fn, ok := cmds[cmd]
	if !ok {
		return 2, fmt.Errorf("unknown command %q", cmd)
	}
	if cmd == "update" || cmd == "install" {
		install.CleanStaging(dir, time.Hour)
	}
	unlock, err := state.Lock(dataDir)
	if err != nil {
		return 1, err
	}
	defer unlock()
	st, err := state.Load(filepath.Join(dataDir, "state.json"), dir)
	if err != nil {
		return 1, err
	}
	if rescan {
		st.Unresolved = nil
	}
	if err := fn(e, st, args); err != nil {
		return 1, err
	}
	return 0, nil
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
	return defaultDataDir(home)
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

// addonRow is one line of the status table.
type addonRow struct {
	Name      string   `json:"name"`
	Installed string   `json:"installed"`
	Latest    string   `json:"latest"`
	Source    string   `json:"source"` // curse or github
	Ref       string   `json:"ref"`
	Status    string   `json:"status"` // checking, ok, update or error
	Adopted   bool     `json:"adopted"`
	Folders   []string `json:"folders"`
	Error     *errInfo `json:"error,omitempty"`
}

// row describes a; c is nil while its check is still running.
func row(a *state.Addon, c *check) addonRow {
	r := addonRow{Name: a.Name, Installed: state.ShortVersion(a.Name, a.Version), Source: a.Source,
		Ref: a.Ref(), Status: "checking", Adopted: a.Adopted(), Folders: a.Folders}
	switch {
	case c == nil:
	case c.err != nil:
		r.Status, r.Error = "error", describe(c.err)
	default:
		r.Latest, r.Status = state.ShortVersion(a.Name, c.rel.Version), "ok"
		if c.outdated(a) {
			r.Status = "update"
		}
	}
	return r
}

func (e *env) checkAll(addons []*state.Addon) map[*state.Addon]check {
	if e.json {
		rows := make([]addonRow, len(addons))
		for i, a := range addons {
			rows[i] = row(a, nil)
		}
		e.emit(checkingEvent{Event: "checking", Addons: rows})
	}
	res := make(map[*state.Addon]check, len(addons))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for _, a := range addons {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			rel, err := e.latest(a.Origin())
			c := check{rel, err}
			mu.Lock()
			defer mu.Unlock()
			res[a] = c
			if e.json { // under mu, so "done" counts up in the order the lines are written
				e.emit(checkedEvent{Event: "checked", Addon: row(a, &c), Done: len(res), Total: len(addons)})
			}
		})
	}
	wg.Wait()
	return res
}

type statusReport struct {
	AddonsDir  string      `json:"addons_dir"`
	BackupsDir string      `json:"backups_dir"`
	Addons     []addonRow  `json:"addons"`
	Untracked  []untracked `json:"untracked"`
	Broken     []broken    `json:"broken"`
	Updates    int         `json:"updates"`
	WowRunning bool        `json:"wow_running"`
}

// untracked is a group of folders wam can't identify, by name prefix.
type untracked struct {
	Root    string   `json:"root"`
	Folders []string `json:"folders"`
}

// broken is a folder unpacked with literal backslashes. CurseID is set when its .toc says
// where it comes from; Ref is that project, or the addon that owns the folder, so the GUI
// can offer to reinstall it.
type broken struct {
	Folder  string `json:"folder"`
	CurseID string `json:"curse_id,omitempty"`
	Ref     string `json:"ref,omitempty"`
}

func cmdStatus(e *env, st *state.State, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("status takes no arguments")
	}
	folders, err := e.sync(st)
	if err != nil {
		return err
	}
	e.noteOldBackups()
	checks := e.checkAll(st.Addons)
	r := statusReport{AddonsDir: e.addonsDir, BackupsDir: e.inst.BackupsDir(), Addons: []addonRow{},
		Untracked: []untracked{}, Broken: []broken{}, WowRunning: install.WowRunning()}
	for _, a := range st.Addons {
		c := checks[a]
		r.Addons = append(r.Addons, row(a, &c))
		if r.Addons[len(r.Addons)-1].Status == "update" {
			r.Updates++
		}
	}
	groups := map[string][]string{}
	for _, f := range folders {
		switch {
		case f.Err != nil:
			continue
		case f.Broken:
			b := broken{Folder: f.Name, CurseID: f.Get("X-Curse-Project-ID")}
			if o := st.Owner(f.Name); b.CurseID != "" {
				b.Ref = source.Curse + ":" + b.CurseID
			} else if o != nil {
				b.Ref = o.Ref()
			}
			r.Broken = append(r.Broken, b)
		case st.Owner(f.Name) == nil:
			root := state.RootName(f.Name)
			groups[root] = append(groups[root], f.Name)
		}
	}
	for root, names := range groups {
		r.Untracked = append(r.Untracked, untracked{root, names})
	}
	sort.Slice(r.Untracked, func(i, j int) bool { return r.Untracked[i].Root < r.Untracked[j].Root })
	if e.json {
		e.data = r
		return nil
	}
	return e.printStatus(r)
}

// noteOldBackups mentions the backups wam 1.0 kept in the data directory; since 1.1 they are
// zips in AddOns-backups and nothing reads the old folder. It is never deleted automatically.
func (e *env) noteOldBackups() {
	if e.dataDir == "" {
		return
	}
	old := filepath.Join(e.dataDir, "backups")
	if entries, err := os.ReadDir(old); err == nil && len(entries) > 0 {
		e.logf("note: backups made by wam 1.0 are still in %s; wam now keeps them in %s, so that folder can be deleted",
			old, e.inst.BackupsDir())
	}
}

func (e *env) printStatus(r statusReport) error {
	tw := tabwriter.NewWriter(e.out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ADDON\tINSTALLED\tLATEST\tSOURCE\tSTATUS")
	guessed := false
	var errs [][2]string
	for _, a := range r.Addons {
		status := e.paint("32", "ok")
		switch a.Status {
		case "error":
			status = e.paint("31", "error")
			errs = append(errs, [2]string{a.Name, a.Error.Message})
		case "update":
			status = e.paint("33", "update")
		}
		installed := a.Installed
		if a.Adopted && a.Error == nil {
			installed += " ~"
			guessed = true
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", a.Name, installed, a.Latest, a.Ref, status)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	var untrackedRows, brokenRows [][2]string
	for _, u := range r.Untracked {
		label := u.Folders[0]
		if len(u.Folders) > 1 {
			label = fmt.Sprintf("%s (+%d folders)", u.Root, len(u.Folders)-1)
		}
		untrackedRows = append(untrackedRows, [2]string{label, fmt.Sprintf("wam track %s <curseforge-url>", u.Folders[0])})
	}
	for _, b := range r.Broken {
		hint := ""
		if b.CurseID != "" {
			hint = fmt.Sprintf("fix: wam install curse:%s && wam remove %q", b.CurseID, b.Folder)
		}
		brokenRows = append(brokenRows, [2]string{b.Folder, hint})
	}
	e.section(e.paint("31", "Errors:"), errs)
	e.section("Not tracked (wam doesn't know where these come from):", untrackedRows)
	e.section(e.paint("31", "Broken (unpacked with literal backslashes; the game can't load them):"), brokenRows)

	e.printf("")
	if guessed {
		e.printf("~ installed outside wam: compared by .toc version until the first update.")
	}
	if r.Updates > 0 {
		e.printf("%s available. Run: wam update", plural(r.Updates, "update"))
	} else {
		e.printf("All %s up to date.", plural(len(r.Addons), "addon"))
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

// updateItem is one addon in an update: its progress while it runs, its outcome after.
type updateItem struct {
	Name  string   `json:"name"`
	From  string   `json:"from"`
	To    string   `json:"to"`
	State string   `json:"state"` // waiting, updating, updated, failed, skipped or would_update
	Error *errInfo `json:"error,omitempty"`
}

type updateReport struct {
	Items   []updateItem `json:"items"`
	DryRun  bool         `json:"dry_run"`
	Updated int          `json:"updated"`
	Failed  int          `json:"failed"` // failed checks included
	Skipped int          `json:"skipped"`
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
	r := updateReport{Items: []updateItem{}, DryRun: dry}
	var todo []*state.Addon
	for _, a := range targets {
		c := checks[a]
		switch {
		case c.err != nil:
			e.logf("%s: %v", a.Name, c.err)
			r.Failed++
		case c.outdated(a):
			todo = append(todo, a)
			r.Items = append(r.Items, updateItem{Name: a.Name, From: state.ShortVersion(a.Name, a.Version),
				To: state.ShortVersion(a.Name, c.rel.Version), State: "waiting"})
		}
	}
	e.data = &r
	if len(todo) == 0 {
		if !e.json {
			e.printf("Everything is up to date.")
		}
		return failedErr(r.Failed)
	}
	if !dry && install.WowRunning() {
		e.warnWowRunning("WoW is running; updated addons load after /reload or a restart")
	}
	progress := func(i int, state string, err error) {
		r.Items[i].State, r.Items[i].Error = state, describe(err)
		if e.json && !dry {
			e.emit(itemEvent{Event: "item", Item: r.Items[i]})
		}
	}
	if e.json && !dry {
		for _, it := range r.Items {
			e.emit(itemEvent{Event: "item", Item: it})
		}
	}
	for i, a := range todo {
		switch {
		case dry:
			progress(i, "would_update", nil)
			continue
		case e.interrupted():
			progress(i, "skipped", nil)
			r.Skipped++
			continue
		}
		progress(i, "updating", nil)
		if err := e.inst.Install(st, a, checks[a].rel, false); err != nil {
			progress(i, "failed", err)
			r.Failed++
		} else if err := st.Save(); err != nil {
			return err
		} else {
			progress(i, "updated", nil)
			r.Updated++
		}
	}
	if !e.json {
		if err := e.printUpdate(r); err != nil {
			return err
		}
	}
	if r.Skipped > 0 {
		return fmt.Errorf("%w: %s not updated", errInterrupted, plural(r.Skipped, "addon"))
	}
	return failedErr(r.Failed)
}

func (e *env) printUpdate(r updateReport) error {
	tw := tabwriter.NewWriter(e.out, 0, 0, 2, ' ', 0)
	for _, it := range r.Items {
		var status string
		switch it.State {
		case "skipped":
			continue
		case "would_update":
			status = e.paint("33", "would update")
		case "failed":
			status = e.paint("31", "failed: "+it.Error.Message)
		default:
			status = e.paint("32", "updated")
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t->\t%s\t%s\n", it.Name, orDash(it.From), it.To, status)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if r.DryRun {
		e.printf("\nDry run: %s, nothing changed.", plural(len(r.Items), "update"))
		return nil
	}
	msg := "Updated " + plural(r.Updated, "addon")
	if r.Failed > 0 {
		msg += fmt.Sprintf(", %d failed", r.Failed)
	}
	if r.Skipped > 0 {
		msg += fmt.Sprintf(", %d skipped", r.Skipped)
	}
	e.printf("\n%s.", msg)
	return nil
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

type installReport struct {
	Name    string   `json:"name"`
	Ref     string   `json:"ref"`
	Version string   `json:"version"`
	Folders []string `json:"folders"`
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
		return badRef{err}
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
	version := state.ShortVersion(a.Name, rel.Version)
	if !e.json {
		e.printf("Installing %s %s", a.Name, version)
	}
	if install.WowRunning() {
		e.warnWowRunning("WoW is running; the addon loads after a restart")
	}
	if err := e.inst.Install(st, a, rel, true); err != nil {
		if isNew {
			st.Remove(a)
		}
		return err
	}
	st.TidyName(a, p)
	e.data = installReport{Name: a.Name, Ref: a.Ref(), Version: version, Folders: a.Folders}
	if !e.json {
		e.printf("Installed folders: %s", strings.Join(a.Folders, ", "))
	}
	return st.Save()
}

type trackReport struct {
	Folder string `json:"folder"`
	Name   string `json:"name"`
	Ref    string `json:"ref"`
}

// folderArg is the AddOns folder a command argument names. "." and ".." would name AddOns
// itself or the Interface directory above it.
func folderArg(s string) (string, error) {
	switch b := filepath.Base(s); b {
	case ".", "..", string(filepath.Separator):
		return "", fmt.Errorf("%q is not a folder in AddOns", s)
	default:
		return b, nil
	}
}

func cmdTrack(e *env, st *state.State, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: wam track <folder> <curseforge-url|slug|curse:id|github-url>")
	}
	folder, err := folderArg(args[0])
	if err != nil {
		return err
	}
	f, err := toc.Read(e.addonsDir, folder)
	if err != nil {
		return err
	}
	p, err := e.lookup(args[1])
	if err != nil {
		return badRef{err}
	}
	st.Disown(folder)
	a := st.Add(p, folder)
	a.Folders = append(a.Folders, folder)
	if a.Adopted() && a.Version == "" {
		a.Version = f.Get("Version")
	}
	e.data = trackReport{Folder: folder, Name: a.Name, Ref: a.Ref()}
	if !e.json {
		e.printf("%s is now tracked as %s (%s)", folder, a.Name, a.Ref())
	}
	return st.Save()
}

type removeReport struct {
	Name    string   `json:"name"`
	Folders []string `json:"folders"`
	Backup  string   `json:"backup"`
}

func cmdRemove(e *env, st *state.State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: wam remove <addon|folder>")
	}
	if _, err := e.sync(st); err != nil {
		return err
	}
	name, folders := args[0], []string(nil)
	a := st.Find(args[0])
	if a != nil {
		name, folders = a.Name, a.Folders
	} else {
		folder, err := folderArg(args[0])
		if err != nil {
			return err
		}
		if _, err := os.Lstat(filepath.Join(e.addonsDir, folder)); err != nil {
			return fmt.Errorf("no addon or folder named %s", args[0])
		}
		folders = []string{folder}
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
	e.data = removeReport{Name: name, Folders: folders, Backup: backup}
	if !e.json {
		e.printf("Removed %s (%s); backup: %s", name, strings.Join(folders, ", "), backup)
	}
	return st.Save()
}

// searchHit is one search result. Ref is empty when the Wago slug matched no CurseForge
// project; then only the Wago page (URL) can help.
type searchHit struct {
	Title     string `json:"title"`
	Ref       string `json:"ref,omitempty"`
	Downloads *int   `json:"downloads"` // Wago's count; null for the exact CurseForge match
	Installed bool   `json:"installed"`
	URL       string `json:"url,omitempty"`
}

type searchReport struct {
	Query           string      `json:"query"`
	Hits            []searchHit `json:"hits"`
	WagoUnavailable bool        `json:"wago_unavailable"`
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
	wg.Go(func() { exact, _ = e.bySlug(state.Slug(text)) })
	slugs, err := e.forever()
	r := searchReport{Query: text, Hits: []searchHit{}, WagoUnavailable: err != nil}
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
		wg.Go(func() { projects[i], _ = e.bySlug(h.Slug) })
	}
	wg.Wait()

	if exact.ID != "" {
		r.Hits = append(r.Hits, searchHit{Title: exact.Title, Ref: exact.Ref(), Installed: st.ByProject(exact) != nil})
	}
	for i, h := range hits {
		downloads := h.Downloads
		switch p := projects[i]; {
		case p.ID == exact.ID && p.ID != "":
		case p.ID != "":
			r.Hits = append(r.Hits, searchHit{Title: p.Title, Ref: p.Ref(), Downloads: &downloads, Installed: st.ByProject(p) != nil})
		default:
			r.Hits = append(r.Hits, searchHit{Title: h.Slug, Downloads: &downloads, URL: "https://addons.wago.io/addons/" + h.Slug})
		}
	}
	if e.json {
		e.data = r
		return nil
	}
	return e.printSearch(r)
}

func (e *env) printSearch(r searchReport) error {
	tw := tabwriter.NewWriter(e.out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tWAGO DOWNLOADS\tINSTALL WITH")
	for _, h := range r.Hits {
		downloads := ""
		if h.Downloads != nil {
			downloads = fmt.Sprint(*h.Downloads)
		}
		how := "wam install " + h.Ref
		switch {
		case h.Ref == "":
			how = "not matched on CurseForge; see " + h.URL
		case h.Installed:
			how = "installed"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", h.Title, downloads, how)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(r.Hits) == 0 {
		e.printf("Nothing found for %q. wam install also takes a CurseForge or GitHub URL.", r.Query)
	} else {
		e.printf("\nCurseForge results may lack a Forever build; wam install says so if they do.")
	}
	return nil
}
