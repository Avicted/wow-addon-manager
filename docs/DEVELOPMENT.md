# Development

Everything runs in Docker through make; `make` lists the commands.

```
make test         go vet + go test, then go test -race, in the Dockerfile's test and race stages
make image        the runtime image (wam:VERSION); its build runs the tests too
make smoke        the image reports VERSION and can change a mounted AddOns directory
make dist         static binary in build/dist/wam
make pre-commit   every hook on every file
make hooks        install the pre-commit git hook
```

With Go installed, `go test ./...` and `go run ./cmd/wam status` work directly.

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
