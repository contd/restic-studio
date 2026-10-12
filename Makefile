# === GLOBALS ===
GO ?= go
ARGS ?=
TEST_FLAGS ?=
SITE_DIR ?= _site

.DEFAULT_GOAL := all
.PHONY: all build test docs api-docs run gen tag major minor patch help

ifneq ($(filter tag,$(MAKECMDGOALS)),)
ifneq ($(words $(MAKECMDGOALS)),2)
$(error Usage: make tag major|minor|patch)
endif
ifneq ($(words $(filter major minor patch,$(MAKECMDGOALS))),1)
$(error Usage: make tag major|minor|patch)
endif
endif

all:
	@echo "🎯 Running all steps..."
	+$(MAKE) test
	+$(MAKE) docs
	+$(MAKE) gen
	+$(MAKE) build

build:
	@echo "🏗️  Running build steps..."
	$(GO) build -trimpath -ldflags="-s -w" -o restic-studio ./cmd/restic-studio

# === Development ===
test:
	@echo "🔧 Running tests..."
	$(GO) test $(TEST_FLAGS) ./...
	@bash scripts/tag_test.sh

docs:
	@echo "💬 Creating the documentation site..."
	$(GO) run ./cmd/docs -output "$(SITE_DIR)"

api-docs:
	@echo "Printing Go package documentation..."
	@set -e; packages="$$($(GO) list ./...)"; \
	for package in $$packages; do \
		printf '\n=== %s ===\n' "$$package"; \
		$(GO) doc "$$package"; \
	done

run:
	@echo "🚀 Running the application..."
	$(GO) run ./cmd/restic-studio $(ARGS)

# === Release ===
tag:
	@echo "🏷️ Running tag steps..."
	@bash scripts/tag.sh $(filter major minor patch,$(MAKECMDGOALS))

major minor patch:
	@echo "🏷️ Validating tag usage..."
	@if [ "$(filter tag,$(MAKECMDGOALS))" != "tag" ]; then \
		printf 'Usage: make tag major|minor|patch\n' >&2; exit 1; \
	fi

# === Capture ===
gen:
	@echo "🎧 Generating screenshots and videos with asciinema and ffmpeg"
	@bash docs/capture.sh

help:
	@echo "🛠️  Usage: make [target]"
	@echo ""
	@echo "Build:"
	@echo "  all                   → 🎯 Run full test, docs, gen and build"
	@echo "  build                 → 🏗️  Build the restic-studio executable"
	@echo "  tag major|minor|patch → 🏷️ Create the next local version tag (does not push)"
	@echo ""
	@echo "Development:"
	@echo "  test                  → 🔧 Run Go tests and version-tagging tests"
	@echo "  docs                  → 💬 Generate the GitHub Pages site in $(SITE_DIR)"
	@echo "  api-docs              → 📂 Print Go package documentation"
	@echo "  run                   → 🚀 Run the app (ARGS='--config /path/to/autorestic.yml')"
	@echo ""
	@echo "Generate Screenshots and Videos:"
	@echo "  gen                   → 🎧 Generate screenshots and videos with asciinema and ffmpeg"
