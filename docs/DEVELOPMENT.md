# Development

Everything runs in Docker through make; `make` lists the commands.

```
make test         go vet + go test, then go test -race, in the Dockerfile's test and race stages
make cover        coverage report in build/coverage/ (coverage.html, coverage.txt)
make image        the runtime image (wam:VERSION); its build runs the tests too
make smoke        the image reports VERSION and can change a mounted AddOns directory
make dist         static binaries in build/dist (wam, wam.exe)
make gui          test, then build/dist/wam-gui and build/dist/wam-gui.exe
make gui-test     cargo fmt --check, clippy and the GUI tests
make release-check  VERSION has a CHANGELOG entry (TAG=vX.Y.Z also checks the tag)
make release-assets versioned binaries and SHA256SUMS in build/release
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
`go build` for the cargo target, so with Go installed a plain `cargo run` works:

```
cd gui
WAM_ADDONS_DIR=/tmp/wam-try/Interface/AddOns WAM_DATA_DIR=/tmp/wam-try/data cargo run
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

## Releasing

1. Set `VERSION` and date the `## [x.y.z]` entry in `CHANGELOG.md`; commit `chore(release): x.y.z`.
2. `make release-check` (and optionally `make release-assets` to build everything locally).
3. Tag and push: `git tag vX.Y.Z && git push origin master vX.Y.Z`.
4. `.github/workflows/release.yml` checks the tag against `VERSION` and the changelog, runs the
   tests, builds the four binaries and publishes a GitHub Release with `SHA256SUMS`.

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
