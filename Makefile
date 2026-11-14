# Everything runs in Docker: the host needs only docker and make.
# (Without Docker, plain Go works too: go run ./cmd/wam status. See README.md.)
#
# Every target is documented as "## Category: description"; `make` lists them.

VERSION := $(shell cat VERSION)
IMAGE   := wam:$(VERSION)
LINT_IMAGE := wam-pre-commit:local

# The AddOns directory wam manages, and where it keeps state and backups.
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
.PHONY: help status update install remove search track wam image dist smoke test pre-commit hooks clean

help: ## Help: list every command
	@awk 'match($$0, /^[a-zA-Z0-9_-]+:.*## /) { \
		target = substr($$0, 1, index($$0, ":") - 1); \
		rest = substr($$0, RLENGTH + 1); \
		sep = index(rest, ": "); \
		print substr(rest, 1, sep - 1) "\t" target "\t" substr(rest, sep + 2) }' $(MAKEFILE_LIST) \
	| LC_ALL=C sort -t '	' -k1,1 -k2,2 \
	| awk -F '\t' '{ cat[NR] = $$1; tgt[NR] = $$2; desc[NR] = $$3; if (!seen[$$1]++) cats++ } \
		END { for (i = 1; i <= NR; i++) { \
			if (cats > 1 && cat[i] != prev) { printf "%s%s\n", (i > 1 ? "\n" : ""), cat[i]; prev = cat[i] } \
			printf "  %-12s %s\n", tgt[i], desc[i] } }'

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

remove: $(IMAGE_STAMP) ## Addons: move NAME=<addon|folder> to the backups
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

dist: ## Build: export the static binary to build/dist/wam
	docker build --target binary --build-arg VERSION=$(VERSION) --output type=local,dest=$(BUILD)/dist .
	@ls -l $(BUILD)/dist/wam

clean: ## Build: delete build/ (binaries, image stamps, tool caches)
	rm -rf $(BUILD)

# ---------------------------------------------------------------------------
# Development.

test: ## Development: go vet, go test and go test -race (always re-run, even if cached)
	docker build --target test --no-cache-filter test --progress plain .
	docker build --target race --no-cache-filter race --progress plain .

smoke: $(IMAGE_STAMP) ## Development: check the image runs and reports VERSION
	@docker run --rm $(IMAGE) --version | grep -qx 'wam $(VERSION)' \
		|| { echo "smoke: --version does not report $(VERSION)"; exit 1; }
	@tmp=$$(mktemp -d); mkdir -p "$$tmp/Interface/AddOns/Foo"; \
		printf '## Version: 1\n' > "$$tmp/Interface/AddOns/Foo/Foo.toc"; \
		docker run --rm $(AS_ME) -v "$$tmp:$$tmp" -e WAM_ADDONS_DIR="$$tmp/Interface/AddOns" -e WAM_DATA_DIR="$$tmp/data" \
			$(IMAGE) remove Foo >/dev/null; rc=$$?; \
		[ $$rc -eq 0 ] && [ ! -e "$$tmp/Interface/AddOns/Foo" ] && [ -d "$$tmp/data/backups" ]; ok=$$?; rm -rf "$$tmp"; \
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
