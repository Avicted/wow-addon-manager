# wam: WoW Forever addon manager

`wam` checks your World of Warcraft Forever addons for newer versions, updates
them, and installs new ones. Addons come from CurseForge and GitHub; only builds
made for the Forever client (1.60.x, interface 16001) are picked.

```
$ make status
ADDON           INSTALLED   LATEST           SOURCE         STATUS
Auctionator     341 ~       346              curse:6124     UPDATE
Leatrix Maps    1.60.28 ~   1.60.31-forever  curse:298842   UPDATE
Questie         v12.9.0     v12.9.0          github:...     ok
Journalator (+3 folders)                                    untracked (wam track Journalator <curseforge-url>)
```

## Requirements

Docker and make. Or, without Docker, Go 1.27:
`go install ./cmd/wam` puts a `wam` binary in `~/go/bin`, and every `make`
command below is then `wam <command>`.

## Usage

Run `make` for the full list of commands.

| Command | Does |
|---|---|
| `make status` | Installed version, latest Forever version and source of every addon |
| `make update` | Update everything that has an update |
| `make update ARGS="Questie -n"` | Only the named addons; `-n` shows what would change |
| `make install REF=<ref>` | Install a new addon |
| `make search Q=details` | Find addons that have Forever builds |
| `make track FOLDER=Journalator REF=curse:453921` | Tell wam where an untracked folder comes from |
| `make remove NAME=<addon>` | Move an addon's folders out of AddOns, into the backups |
| `make wam ARGS=help` | Any other `wam` command |

`<ref>` is a CurseForge URL (`https://www.curseforge.com/wow/addons/details`),
slug (`details`) or project id (`curse:61284`), or a GitHub repository
(`github:Questie/Questie`, or its URL).

### Where things are

| | Default | Override |
|---|---|---|
| AddOns | `~/Games/battlenet/drive_c/Program Files (x86)/World of Warcraft/_forever_/Interface/AddOns` | `ADDONS_DIR=...` (make) or `WAM_ADDONS_DIR` |
| State and backups | `~/.local/share/wam` | `DATA_DIR=...` (make) or `WAM_DATA_DIR` |

`GITHUB_TOKEN`, if set, raises GitHub's API limit of 60 requests an hour.

### Addons you installed before wam

The first `status` adopts the folders already in AddOns: it reads each
folder's `.toc` for a CurseForge project id or URL, or matches the folder name
to a CurseForge project. Until wam has updated an addon once it doesn't know
the exact installed release, so it compares the `.toc` version with the latest
release name; those rows are marked `~`. Folders it can't place are listed as
`untracked`, with the `track` command to fix them.

### Safety

- Replaced and removed folders go to `<data>/backups/<addon>/<time>/`; the most
  recent backup per addon is kept. If an install fails part-way, the old folders
  are put back.
- `WTF/` (settings and saved variables) is never touched.
- Updated addons load after `/reload` or a restart. Run natively, wam warns when
  WoW is running; in Docker it can't see the game's process.

## Limits

- CurseForge is read through its website's API, which needs no key but isn't
  the documented API and may change. Failures are reported per addon.
- CurseForge search needs a key, so `search` checks the query as a CurseForge
  name and searches Wago's list of Forever addons. A URL always works with
  `install`.
- Wago downloads need an API key and aren't supported.

## More

- [docs/DESIGN.md](docs/DESIGN.md): how wam finds, compares and installs addons.
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md): layout, tests, hooks, CI.
