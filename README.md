# wam: WoW Forever addon manager

`wam` checks your World of Warcraft Forever addons for newer versions, updates
them, and installs new ones from CurseForge and GitHub. Only builds made for the
Forever client (1.60.x) are picked.

```
$ make status
ADDON         INSTALLED  LATEST           SOURCE        STATUS
Auctionator   341 ~      346              curse:6124    update
Leatrix Maps  1.60.28 ~  1.60.31-forever  curse:298842  update
Questie       v12.9.0    v12.9.0          github:...    ok

Not tracked (wam doesn't know where these come from):
  Journalator (+3 folders)  wam track Journalator <curseforge-url>

2 updates available. Run: wam update
```

## Quick start

1. Install Docker and make.
2. Clone this repository and `cd` into it.
3. If WoW isn't in the default Battle.net/Wine location, point wam at your
   AddOns folder:
   `export WAM_ADDONS_DIR="/path/to/_forever_/Interface/AddOns"`
4. `make status` shows what's installed and what has an update.
5. `make update` updates everything. Then `/reload` in game.
6. `make search Q=details` finds an addon, `make install REF=details` installs it.

No Docker? With Go 1.27, `go install ./cmd/wam` gives you a `wam` binary, and
each `make <command>` below is `wam <command>`.

## Commands

Run `make` for the full list.

| Command | Does |
|---|---|
| `make status` | Installed and latest version of every addon |
| `make update` | Update everything that has an update |
| `make update ARGS="Questie -n"` | Only the named addons; `-n` shows what would change |
| `make install REF=<ref>` | Install a new addon |
| `make search Q=<text>` | Find addons that have Forever builds |
| `make track FOLDER=<folder> REF=<ref>` | Tell wam where an untracked folder comes from |
| `make remove NAME=<addon>` | Move an addon's folders to the backups |

`<ref>` is a CurseForge URL, slug (`details`) or project id (`curse:61284`), or
a GitHub repository (`github:Questie/Questie`).

## Good to know

- Rows marked `~` were installed before wam; they're compared by `.toc`
  version until wam updates them once.
- Folders under "Not tracked" need `make track` so wam can update them.
- Replaced and removed addons are backed up in `~/.local/share/wam/backups`
  (override with `WAM_DATA_DIR`). `WTF/` (your settings) is never touched.
- Set `GITHUB_TOKEN` to raise GitHub's 60 requests an hour limit.
- Output is colored on a terminal; set `NO_COLOR=1` to turn it off.

More: [how it works and its limits](docs/DESIGN.md), [development](docs/DEVELOPMENT.md).
