# Design

## Packages

```
cmd/wam            entry point; sets the version from -ldflags
internal/cli       commands, flags, table output
internal/toc       reads AddOns: one Folder per directory, with its .toc header
internal/source    CurseForge, cfwidget, GitHub, Wago: resolve a ref, find the latest Forever build, download
internal/state     state.json: which project owns which folders, at which release; adoption
internal/install   unzip, swap into AddOns, backup, rollback
```

Dependencies point one way: `cli` → `install` → `state` → `source`, `toc`.
Only the standard library is used.

## Finding Forever builds

- **CurseForge**: `www.curseforge.com/api/v1/mods/{id}/files`, newest first.
  The endpoint ignores its game-version filter, so wam filters itself: a file is
  for Forever if its `gameVersionTypeIds` contain `88568` or a game version starts
  with `1.60.`. Release files win over betas. Downloads go through
  `/files/{id}/download`, which redirects to the CDN.
- **cfwidget** (`api.cfwidget.com`) maps CurseForge slugs to project ids and
  titles; the CurseForge website API refuses that without a key.
- **GitHub**: the latest release. When it has a BigWigs-packager `release.json`,
  the file whose metadata lists flavor `forever` is used; otherwise the only
  `.zip`, or the only one with "forever" in its name.
- **Wago**: only its keyless list of addons with Forever builds, for `search`.

## The .toc file

A folder may hold several `.toc` files, one per game flavor. The Forever client
prefers `_Forever`/`-Forever`, then `_Camelot`/`-Camelot` (the internal name),
then the plain `<Folder>.toc`; wam reads them in that order. Only the first
16 KiB is read: QuestieDB's is tens of megabytes of data after the header.

## State and adoption

`state.json` holds one entry per project: source, project id, installed release
(CurseForge file id or GitHub tag), version string and the folders it owns. It
records the AddOns path it belongs to and refuses any other.

Folders wam didn't install are adopted on every command, cheaply after the
first time:

1. `X-Curse-Project-ID` in the `.toc`.
2. Otherwise, by name prefix: an unowned `ForeverArtisan_Alchemy` joins the
   adopted addon owning `ForeverArtisan_Core` (the part before the first `_`,
   at least 4 characters). This applies to adopted addons only: an addon wam
   installed owns exactly the folders its zip contained, so a folder it dropped
   (QuestieDB, when CurseForge's Questie stopped shipping it) shows as untracked
   instead of silently never updating.
3. Otherwise `X-Website` pointing at CurseForge, or the folder name as a slug,
   accepted only if the project title matches the `.toc` title.
4. Otherwise the folder is remembered as unresolved, so it isn't looked up
   again (`-rescan` retries). Only a definitive "not found" (`source.ErrNotFound`)
   counts: if a lookup failed for any other reason (network down, a 5xx), the
   folder is left alone and the next run tries again.

`## Dependencies` is deliberately not used for grouping: Guidelime depends on
Questie and Journalator_Display on Auctionator, but they're separate projects.

For adopted addons the installed release is unknown, so "up to date" means the
`.toc` version appears as a whole token in the latest release name
(`12.6.4` in `v12.6.4`, not `34` in `346`). After wam's first update the
comparison is by release id.

## Installing

1. Download to a temporary directory.
2. Unzip into a staging directory beside AddOns (same filesystem, so the swap is
   a rename). Backslashes in entry names become `/` (Windows-made zips unpacked
   on Linux otherwise give files named `Sub\file.lua`, which the game can't
   load); absolute paths and `..` are refused; a single wrapper directory
   around the addon folders is stripped; over 2 GiB unpacked is refused.
3. Zip each existing folder of the same name into `Interface/AddOns-backups/<addon>/<time>.zip`
   (a failure here stops the install with nothing changed). Then rename the old folders into a
   second staging directory and the new ones into AddOns. Any failure moves the new ones out,
   the old ones back and deletes the zip; on success the staging directories are deleted and
   only the newest zip is kept.
4. Folders another addon owns: `update` refuses (two addons would keep
   overwriting each other); an explicit `install` takes them over, which is how
   an addon switches source (`install github:Questie/Questie`).

## Running safely

- One wam at a time: every command takes an exclusive lock on
  `state.lock` in the data directory (`flock` on Unix, a `CreateFile` with share mode 0 on
  Windows) and fails at once if another holds it.
- The first Ctrl-C (or SIGTERM) lets the addon being installed finish and skips
  the rest of `update`, so AddOns is never left half swapped; a second one ends
  wam. Staging directories a killed wam left beside AddOns are removed after an
  hour.
- Network errors, 429 and 502-504 are retried twice (1 s, then 3 s, or the
  server's `Retry-After` up to 10 s). Other statuses are answers and aren't.
- A CurseForge project's file list is fetched once per run.
- A folder that can't be read is listed with a warning; it is never adopted,
  nor forgotten if it was tracked.
- When a GitHub release publishes a SHA-256 digest for the asset, the download
  must match it before it is unpacked.

## Docker

The image is distroless with only the static binary. The Makefile mounts the
whole `Interface` directory (staging happens beside AddOns) and the data
directory at their host paths, so `state.json` is the same file whether wam
runs natively or in Docker, and runs the container as the host user so files it
writes stay the user's.

## Limits

- CurseForge is read through its website's API, which needs no key but isn't
  the documented API and may change. Failures are reported per addon.
- CurseForge search needs a key, so `search` checks the query as a CurseForge
  name and searches Wago's list of Forever addons. A URL always works with
  `install`.
- Wago downloads need an API key and aren't supported.
- wam warns when WoW is running if run natively (it looks for `wow.exe` in `/proc`, or in the
  Windows process list); in Docker it can't see the game's process.

## Windows

The core is stdlib-only Go and builds for Windows; the three OS-specific parts live in
`_unix.go` / `_windows.go` files:

- **Lock**: `state.Lock` (see above). A second wam gets `ErrLocked` on both systems.
- **Same-drive moves**: staging directories and backups live in `Interface/` beside AddOns, so
  every move is a rename on one filesystem, wherever the game is installed.
- **WoW running**: `install.WowRunning` scans `/proc` on Linux and a ToolHelp snapshot on Windows.
- **Default paths**: AddOns is `C:\Program Files (x86)\World of Warcraft\_forever_\Interface\AddOns`;
  state is in `%LOCALAPPDATA%\wam`; backups are in `Interface\AddOns-backups`. `WAM_ADDONS_DIR` and `WAM_DATA_DIR` win everywhere.
- Windows refuses to move folders that have open files, so updating while the game runs can fail
  there; the rollback restores the old folders and wam warns before it starts.

`go vet` runs with `GOOS=windows` in the Dockerfile's test stage, so this can't regress.

## JSON mode and the desktop app

`wam -json <command>` prints one JSON object per line on stdout and nothing else; the last line is
always a `result`. Messages for people (`log`) become events too. The event shapes are documented
at the top of `internal/cli/json.go`:

| `event` | Meaning |
|---|---|
| `log` | `level` info or warning, `message`, optional `code` (`wow_running`) |
| `checking` / `checked` | update checks starting, and each one finishing (`done` of `total`) |
| `item` | per-addon progress of an update or install |
| `result` | `ok`, `data` (the report of that command) or `error` `{message, code, ...}` |

Error `code` is one of `locked`, `addons_dir`, `cancelled`, `rate_limit`, `http`, `network`,
`bad_ref`, `not_found`, `in_use`, `disk_full`, `permission`, `other`, so a front end can explain
it in plain words. Each command builds a report struct and renders it as text or JSON.

**Cancel**: Windows has no SIGINT for a child process, so in JSON mode a `cancel` line on stdin
does what the first Ctrl-C does: the addon being installed finishes and the rest are skipped. EOF
on stdin is ignored, so scripts using `-json` are unaffected.

`gui/` is a Rust (egui) app that embeds the `wam` binary for its own platform, unpacks it to
`<data dir>/bin/wam-<version>[.exe]` and runs it as a subprocess with `-json`. It has no addon
logic of its own, so the CLI and the app always do the same thing to the same `state.json`. A
subprocess instead of FFI keeps Go's cgo out of the build and lets both run side by side. The
plan and the design handover are in [GUI-PLAN.md](GUI-PLAN.md) and [gui-design/](gui-design/).
