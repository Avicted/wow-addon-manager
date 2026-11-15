# Everything runs in Docker: the host needs only docker and make.
# (Without Docker, plain Go works too: go run ./cmd/wam status. See README.md.)
#
# Every target is documented as "## Category: description"; `make` lists them.

VERSION := $(shell cat VERSION)
IMAGE   := wam:$(VERSION)
LINT_IMAGE := wam-pre-commit:local

# The AddOns directory wam manages, and where it keeps state.
ADDONS_DIR ?= $(or $(WAM_ADDONS_DIR),$(HOME)/Games/battlenet/drive_c/Program Files (x86)/World of Warcraft/_forever_/Interface/AddOns)
DATA_DIR   ?= $(or $(WAM_DATA_DIR),$(HOME)/.local/share/wam)

BUILD   := build
SOURCES := go.mod $(shell find cmd internal -name '*.go')
# Stamps record that an image is current, so `make status` doesn't rebuild every time.
IMAGE_STAMP := $(BUILD)/.image-$(VERSION)
LINT_STAMP  := $(BUILD)/.pre-commit-image

# Containers run as you, so files they write (AddOns, state, caches) stay yours.
AS_ME := --user $(shell id -u):$(shell id -g)
TTY   := $(shell [ -t 0 ] && echo -t)

.DEFAULT_GOAL := help
.PHONY: help status update install remove search track wam image dist gui gui-run gui-dev gui-test release release-check version-check new-version undo-version release-assets release-notes smoke test check cover pre-commit hooks clean

help: ## Help: list every command
	@color=; [ -t 1 ] && [ -z "$${NO_COLOR:-}" ] && color=1; \
	awk 'match($$0, /^[a-zA-Z0-9_-]+:.*## /) { \
		target = substr($$0, 1, index($$0, ":") - 1); \
		rest = substr($$0, RLENGTH + 1); \
		sep = index(rest, ": "); \
		print substr(rest, 1, sep - 1) "\t" target "\t" substr(rest, sep + 2) }' $(MAKEFILE_LIST) \
	| LC_ALL=C sort -t '	' -k1,1 -k2,2 \
	| awk -F '\t' -v color="$$color" \
		'BEGIN { if (color) { head = "\033[1;35m"; name = "\033[36m"; off = "\033[0m" } } \
		{ cat[NR] = $$1; tgt[NR] = $$2; desc[NR] = $$3; if (!seen[$$1]++) cats++ } \
		END { for (i = 1; i <= NR; i++) { \
			if (cats > 1 && cat[i] != prev) { printf "%s%s%s%s\n", (i > 1 ? "\n" : ""), head, cat[i], off; prev = cat[i] } \
			printf "  %s%-14s%s %s\n", name, tgt[i], off, desc[i] } }'

# ---------------------------------------------------------------------------
# Running wam. The whole Interface directory is mounted, at the same path as on
# the host: wam stages downloads beside AddOns, and state.json records the path.

define run-wam
	@mkdir -p "$(DATA_DIR)"
	@iface="$$(dirname "$(ADDONS_DIR)")"; \
	docker run --rm -i $(TTY) $(AS_ME) \
		-v "$$iface:$$iface" -v "$(DATA_DIR):$(DATA_DIR)" \
		-e WAM_ADDONS_DIR="$(ADDONS_DIR)" -e WAM_DATA_DIR="$(DATA_DIR)" -e GITHUB_TOKEN \
		$(IMAGE) $(1)
endef

status: $(IMAGE_STAMP) ## Addons: show installed addons and available updates
	$(call run-wam,status)

update: $(IMAGE_STAMP) ## Addons: update everything, or ARGS="Questie -n" (names, dry run)
	$(call run-wam,update $(ARGS))

install: $(IMAGE_STAMP) ## Addons: install REF=<curseforge-url|slug|curse:id|github-url>
	$(if $(REF),,$(error usage: make install REF=<curseforge-url|slug|curse:id|github-url>))
	$(call run-wam,install '$(REF)')

remove: $(IMAGE_STAMP) ## Addons: remove NAME=<addon|folder>, keeping a zip backup
	$(if $(NAME),,$(error usage: make remove NAME=<addon|folder>))
	$(call run-wam,remove '$(NAME)')

search: $(IMAGE_STAMP) ## Addons: find addons with Forever builds, Q=<text>
	$(if $(Q),,$(error usage: make search Q=<text>))
	$(call run-wam,search '$(Q)')

track: $(IMAGE_STAMP) ## Addons: record FOLDER=<folder> as coming from REF=<ref>
	$(if $(and $(FOLDER),$(REF)),,$(error usage: make track FOLDER=<folder> REF=<ref>))
	$(call run-wam,track '$(FOLDER)' '$(REF)')

wam: $(IMAGE_STAMP) ## Addons: run any wam command, ARGS="..." (e.g. ARGS=help)
	$(call run-wam,$(ARGS))

# ---------------------------------------------------------------------------
# Building. Tests run inside every image build, so a failing test blocks both.

$(IMAGE_STAMP): $(SOURCES) Dockerfile VERSION
	docker build -t $(IMAGE) --build-arg VERSION=$(VERSION) .
	@mkdir -p $(BUILD) && touch $@

image: $(IMAGE_STAMP) ## Build: build the wam image (wam:VERSION)

dist: ## Build: export the static binaries to build/dist (wam, wam.exe for Windows)
	docker build --target binary --build-arg VERSION=$(VERSION) --output type=local,dest=$(BUILD)/dist .
	@ls -l $(BUILD)/dist/wam $(BUILD)/dist/wam.exe

gui: ## Build: test, then export the desktop app to build/dist (wam-gui, wam-gui.exe)
	docker build -f Dockerfile.gui --target gui --output type=local,dest=$(BUILD)/dist .
	@ls -l $(BUILD)/dist/wam-gui $(BUILD)/dist/wam-gui.exe

gui-run: gui ## Addons: build and open the desktop app, the same binary users download
	@WAM_ADDONS_DIR="$(ADDONS_DIR)" WAM_DATA_DIR="$(DATA_DIR)" $(BUILD)/dist/wam-gui

# With TAG it also checks the tag sits on the commit make release made for VERSION.
release-check: ## Build: check VERSION has a CHANGELOG section (and matches TAG=vX.Y.Z if given)
	@$(if $(TAG),[ "$(TAG)" = "v$(VERSION)" ] || { echo "release-check: tag $(TAG) does not match VERSION $(VERSION)"; exit 1; }; \
		[ "$$(git log -1 --format=%s)" = "chore(release): $(VERSION)" ] || \
			{ echo "release-check: $(TAG) is not on a 'chore(release): $(VERSION)' commit; release with make release"; exit 1; };) \
		[ -n "$$($(MAKE) -s release-notes)" ] || { echo "release-check: CHANGELOG.md has no entry for $(VERSION)"; exit 1; }
	@echo "release-check: OK ($(VERSION))"

release-notes:
	@./scripts/changelog.sh --section "$(VERSION)"

# The commits since the last v* tag by default; CI passes the range of the push.
LAST_TAG := $(shell git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null)
RANGE    ?= $(if $(LAST_TAG),$(LAST_TAG)..HEAD,HEAD)

version-check: ## Build: check every VERSION change in RANGE (default: since the last tag) came from make release
	@./scripts/check-version-bumps.sh "$(RANGE)"

# make new-version BUMP=<major|minor|patch>
# Bumps VERSION and adds a CHANGELOG.md section generated from the commits since the last tag,
# then runs release-check. It needs a clean tree, so the changelog matches what ships. It does not
# commit or tag: review the diff, commit `chore(release): x.y.z`, tag and push.
PREV_VERSION := $(BUILD)/.prev-version

new-version: ## Build: bump VERSION (BUMP=major|minor|patch) and add its CHANGELOG section
	@case "$(BUMP)" in major|minor|patch) ;; \
		*) echo "usage: make new-version BUMP=<major|minor|patch>"; exit 2 ;; esac
	@[ -z "$$(git status --porcelain)" ] || \
		{ echo "new-version: commit your work first; the changelog is built from commits"; exit 1; }
	@new=$$(awk -F. -v b="$(BUMP)" '{ \
		if (b == "major") { $$1++; $$2 = 0; $$3 = 0 } \
		else if (b == "minor") { $$2++; $$3 = 0 } \
		else { $$3++ } \
		printf "%d.%d.%d", $$1, $$2, $$3 }' VERSION); \
	mkdir -p $(BUILD); cp VERSION $(PREV_VERSION); \
	echo "$$new" > VERSION; \
	echo "new-version: $(VERSION) -> $$new"; \
	./scripts/changelog.sh "$$new"
	@$(MAKE) --no-print-directory release-check

# make undo-version puts back the version new-version replaced and removes the changelog section
# it added. Only those are rewritten, so unrelated edits survive, and it works after the bump was
# committed too (leaving a change to commit).
undo-version: ## Build: undo the last new-version
	@[ -f $(PREV_VERSION) ] || { echo "undo-version: no bump to undo (run new-version first)"; exit 1; }
	@old=$$(cat $(PREV_VERSION)); \
	./scripts/changelog.sh --remove "$(VERSION)"; \
	echo "$$old" > VERSION; rm -f $(PREV_VERSION); \
	echo "undo-version: $(VERSION) -> $$old"
	@$(MAKE) --no-print-directory release-check

# make release BUMP=<major|minor|patch> is the one way to ship a version: new-version, then the
# commit `chore(release): x.y.z` and the tag vx.y.z, then (after you say y) the push that starts
# .github/workflows/release.yml. CI rejects version bumps made any other way (version-check).
release: ## Build: ship a version (BUMP=major|minor|patch): bump, commit, tag, then push on y
	@[ "$$(git rev-parse --abbrev-ref HEAD)" = master ] || { echo "release: switch to master first"; exit 1; }
	@$(MAKE) --no-print-directory new-version BUMP=$(BUMP)
	@set -e; v=$$(cat VERSION); \
	git add VERSION CHANGELOG.md; \
	WAM_RELEASE=1 git commit -q -m "chore(release): $$v"; \
	git tag -a "v$$v" -m "v$$v"; \
	echo; $(MAKE) -s --no-print-directory release-notes; echo; \
	printf 'release: push master and v%s to origin? [y/N] ' "$$v"; \
	read -r ans || ans=; \
	case "$$ans" in \
	y|Y) git push --atomic --no-follow-tags origin master "v$$v" ;; \
	*) echo "release: not pushed. To push: git push --atomic --no-follow-tags origin master v$$v"; \
	   echo "release: to undo:  git tag -d v$$v && git reset --hard HEAD~1" ;; \
	esac

# The names carry no version, so .../releases/latest/download/<name> always fetches the newest.
release-assets: dist gui release-check ## Build: release binaries and SHA256SUMS in build/release
	rm -rf $(BUILD)/release && mkdir -p $(BUILD)/release
	cp $(BUILD)/dist/wam $(BUILD)/release/wam-linux-amd64
	cp $(BUILD)/dist/wam.exe $(BUILD)/release/wam-windows-amd64.exe
	cp $(BUILD)/dist/wam-gui $(BUILD)/release/wam-gui-linux-amd64
	cp $(BUILD)/dist/wam-gui.exe $(BUILD)/release/wam-gui-windows-amd64.exe
	cd $(BUILD)/release && sha256sum * > SHA256SUMS && cat SHA256SUMS

clean: ## Build: delete build/ (binaries, image stamps, tool caches)
	rm -rf $(BUILD)

# ---------------------------------------------------------------------------
# Development.

# Everything CI runs, in one go. Each step is its own target, so a failure names the step.
# The steps are independent, so they run in parallel (-j); -O prints each step's output as one
# block when it finishes instead of interleaving the logs.
CHECKS := test gui-test smoke dist gui pre-commit release-check version-check

check: ## Development: run everything CI runs, in parallel (tests, GUI, smoke, builds, hooks, release and version checks)
	@$(MAKE) --no-print-directory -j -O $(CHECKS)

test: ## Development: go vet, go test and go test -race (always re-run, even if cached)
	docker build --target test --no-cache-filter test --progress plain .
	docker build --target race --no-cache-filter race --progress plain .

gui-dev: ## Development: run the desktop app from source with cargo run (needs Rust and Go on the host)
	@cd gui && WAM_ADDONS_DIR="$(ADDONS_DIR)" WAM_DATA_DIR="$(DATA_DIR)" cargo run

gui-test: ## Development: cargo fmt --check, clippy and the GUI tests (always re-run)
	docker build -f Dockerfile.gui --target test --no-cache-filter test --progress plain .

cover: ## Development: test coverage report in build/coverage/ (coverage.html, .txt, .out)
	docker build --target coverage --no-cache-filter cover --output type=local,dest=$(BUILD)/coverage .
	@tail -n 1 $(BUILD)/coverage/coverage.txt
	@echo "cover: open $(BUILD)/coverage/coverage.html"

smoke: $(IMAGE_STAMP) ## Development: check the image runs and reports VERSION
	@docker run --rm $(IMAGE) --version | grep -qx 'wam $(VERSION)' \
		|| { echo "smoke: --version does not report $(VERSION)"; exit 1; }
	@tmp=$$(mktemp -d); mkdir -p "$$tmp/Interface/AddOns/Foo"; \
		printf '## Version: 1\n' > "$$tmp/Interface/AddOns/Foo/Foo.toc"; \
		docker run --rm $(AS_ME) -v "$$tmp:$$tmp" -e WAM_ADDONS_DIR="$$tmp/Interface/AddOns" -e WAM_DATA_DIR="$$tmp/data" \
			$(IMAGE) remove Foo >/dev/null; rc=$$?; \
		[ $$rc -eq 0 ] && [ ! -e "$$tmp/Interface/AddOns/Foo" ] && ls "$$tmp"/Interface/AddOns-backups/Foo/*.zip >/dev/null 2>&1; ok=$$?; rm -rf "$$tmp"; \
		[ $$ok -eq 0 ] || { echo "smoke: remove in the container failed (exit $$rc)"; exit 1; }
	@echo "smoke: OK"

$(LINT_STAMP): Dockerfile.pre-commit
	docker build -t $(LINT_IMAGE) -f Dockerfile.pre-commit .
	@mkdir -p $(BUILD) && touch $@

# prek in its container. The repository is mounted at its own path so the
# paths git passes to hooks (GIT_INDEX_FILE during `git commit -a`) resolve.
define run-prek
	@mkdir -p $(BUILD)/cache
	@docker run --rm -i $(TTY) $(AS_ME) -v "$(CURDIR):$(CURDIR)" -w "$(CURDIR)" -e GIT_INDEX_FILE \
		-e PREK_HOME="$(CURDIR)/$(BUILD)/cache/prek" -e GOCACHE="$(CURDIR)/$(BUILD)/cache/go-build" \
		-e GOMODCACHE="$(CURDIR)/$(BUILD)/cache/go-mod" -e GOLANGCI_LINT_CACHE="$(CURDIR)/$(BUILD)/cache/golangci-lint" \
		$(LINT_IMAGE) $(1)
endef

pre-commit: $(LINT_STAMP) ## Development: run every pre-commit hook on every file
	$(call run-prek,run --all-files --show-diff-on-failure)

# Used by the git hooks in scripts/git-hooks; not listed.
prek: $(LINT_STAMP)
	$(call run-prek,$(ARGS))

hooks: $(LINT_STAMP) ## Development: install the pre-commit git hook
	git config core.hooksPath scripts/git-hooks
	@echo "hooks: git now runs scripts/git-hooks (prek in Docker)"
