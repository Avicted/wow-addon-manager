# Development

Everything runs in Docker through make; `make` lists the commands.

```
make test         go vet + go test, then go test -race, in the Dockerfile's test and race stages
make cover        coverage report in build/coverage/ (coverage.html, coverage.txt)
make image        the runtime image (wam:VERSION); its build runs the tests too
make smoke        the image reports VERSION and can change a mounted AddOns directory
make dist         static binaries in build/dist (wam, wam.exe)
make gui          test, then build/dist/wam-gui and build/dist/wam-gui.exe
make gui-run      make gui, then open build/dist/wam-gui (what users get)
make gui-dev      run the app from source with cargo run (needs Rust and Go on the host)
make gui-test     cargo fmt --check, clippy and the GUI tests
make check        everything CI runs: the tests above, smoke, dist, gui, pre-commit, release-check, version-check
make release BUMP=minor   ship a version: bump, changelog, commit, tag, push (see Releasing)
make new-version BUMP=minor   only the bump and the CHANGELOG section (make release runs it)
make undo-version take the last new-version back
make release-check  VERSION has a CHANGELOG entry (TAG=vX.Y.Z also checks the tag and its commit)
make version-check  every VERSION change since the last tag is a make release commit
make release-assets release binaries and SHA256SUMS in build/release
make pre-commit   every hook on every file
make hooks        install the pre-commit git hook
```

With Go installed, `go test ./...` (`-cover` for coverage) and `go run ./cmd/wam status` work directly.

The parsers of downloaded files have fuzz tests; their seeds run with `go test`. To
fuzz for a while:

```
go test -run '^$' -fuzz FuzzExtractZip -fuzztime 60s ./internal/install
go test -run '^$' -fuzz FuzzParse -fuzztime 60s ./internal/toc
```

Tests are offline: CurseForge and GitHub are `httptest` servers, the AddOns
directory is a temporary one, and `install` swaps and rolls back on real files.
Point a run at a copy of your AddOns directory before trying changes on the
real one:

```
cp -a ".../Interface" /tmp/wam-try/
make status ADDONS_DIR=/tmp/wam-try/Interface/AddOns DATA_DIR=/tmp/wam-try/data
```

## Desktop app

`gui/` is a cargo crate. Its `build.rs` embeds `wam`: `$WAM_BIN` if set, otherwise it runs
`go build` for the cargo target, so with Go installed a plain `cargo run` works. `make gui-dev`
does that with `ADDONS_DIR` and `DATA_DIR` like the other targets (`make gui-run` opens the
binary built by `make gui` instead). To try it on a copy of your addons:

```
make gui-dev ADDONS_DIR=/tmp/wam-try/Interface/AddOns DATA_DIR=/tmp/wam-try/data
```

`make gui` does the Linux and Windows builds in `Dockerfile.gui` (Debian bookworm for an old
glibc; MinGW for Windows). The UI tests run headlessly; one test drives a real `wam` against a
copy of an AddOns directory and is ignored by default:

```
WAM_E2E_DIR=/tmp/wam-try cargo test real_wam -- --ignored --nocapture
```

Windows can be checked under Wine: `cargo build --release --target x86_64-pc-windows-gnu` (with
`gcc-mingw-w64` and the rustup target), then `wine target/x86_64-pc-windows-gnu/release/wam-gui.exe`.
The design handover (tokens, mockups, copy) is in `docs/gui-design/`.

The app icon is `gui/assets/icon.svg`; `icon.png` (window icon, Linux launcher icon) and
`icon.ico` (embedded in the Windows `.exe` by `build.rs`) are committed renders of it:

```
cd gui/assets
rsvg-convert -w 256 -h 256 icon.svg -o icon.png
magick icon.png -define icon:auto-resize=256,128,64,48,32,24,16 icon.ico
```

On Linux the app writes `applications/wam.desktop` and `icons/hicolor/256x256/apps/wam.png`
under `$XDG_DATA_HOME` at start-up, because Wayland takes the taskbar icon from the `.desktop`
file that matches the app id (`wam`).

## Releasing

Every version goes out through one command, on `master` with a clean tree:

```
make release BUMP=<major|minor|patch>
```

It runs `make new-version`, which bumps `VERSION`, adds a `## [x.y.z]` section to
`CHANGELOG.md` from the conventional commits since the last tag (`scripts/changelog.sh`) and
runs `make release-check`. Then it commits `chore(release): x.y.z`, tags `vx.y.z`, prints the
release notes and asks before pushing `master` and the tag. Pushing the tag starts
`.github/workflows/release.yml`, which checks the tag against `VERSION`, its commit and the
changelog, runs the tests, builds the four binaries and publishes a GitHub Release with
`SHA256SUMS`. That release becomes the one the README's download links fetch.

To edit the changelog first, answer `n`, edit `CHANGELOG.md`, then
`git commit --amend -a --no-edit`, `git tag -f -a vX.Y.Z -m vX.Y.Z` and the push it printed.
To drop the release instead, use the undo it printed. `make release-assets` builds the release
files locally if you want to try them first.

Bumps made any other way are refused. The pre-commit hook rejects a staged `VERSION` outside
`make release`. CI's `make version-check` fails any commit that changes `VERSION` unless it is
exactly `chore(release): x.y.z`, changes only `VERSION` and `CHANGELOG.md`, moves one major,
minor or patch step and has that changelog section. `release-check TAG=` fails a tag that isn't
on such a commit.

## CI

`.github/workflows/ci.yml` runs `make check` on every push and pull request, plus a check that
commit subjects follow Conventional Commits and that the push's version bumps came from
`make release` (`make version-check`). `release.yml` runs on a `v*` tag.

After the checks pass on master, a `coverage` job runs `make cover` and force-pushes
`coverage.json` (built by `scripts/coverage-badge.sh`) to a single-commit orphan `badges` branch.
The README's coverage badge reads that file through shields.io, so it needs a public repository
and says "invalid" until the first master push has run the job. Coverage is for the Go code only.

Jobs prefer the self-hosted runner (labels `self-hosted`, `linux`), which needs Docker, `make`
and a user allowed to run `docker`. GitHub cannot fall back by itself when that runner is
offline: set the repository variable `CI_RUNNER` to `"ubuntu-latest"` (with the quotes; Settings,
Secrets and variables, Actions, Variables) to use GitHub's runners, and delete it to go back.
Don't point a self-hosted runner at a public repository without requiring approval for pull
requests from forks (Settings, Actions, General): they would run on your machine.

## Hooks

`make hooks` sets `core.hooksPath` to `scripts/git-hooks`. The pre-commit hook runs prek
in the image built from `Dockerfile.pre-commit`, so nothing but Docker is
needed:

| Stage | Hooks |
|---|---|
| pre-commit | prek built-ins (whitespace, end of file, YAML, merge markers, large files, case conflicts, shebangs, line endings), gitleaks, golangci-lint fmt + lint (`.golangci.yml`), hadolint |

`Dockerfile.pre-commit` copies each tool's binary from its official image,
pinned by digest, and `.pre-commit-config.yaml` uses each tool's published
`*-system` hook, so nothing is downloaded or built when a hook first runs.

Caches (prek, Go, golangci-lint) live in `build/cache`; `make clean` removes them.
Git worktrees aren't supported by the hooks: their `.git` points outside the
mounted directory.

## Versions and pins

- `VERSION` is the release version; `make image` and `make dist` stamp it into
  the binary (`wam --version`).
- Every image is pinned by digest. Dependabot proposes updates for both
  Dockerfiles, the GitHub Actions and Go modules. When bumping golangci-lint,
  its image must be built with at least the Go version in `go.mod`.

## Commits

Follow [Conventional Commits](https://www.conventionalcommits.org/).
