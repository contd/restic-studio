# === GLOBALS ===
GO ?= go
ARGS ?=

.DEFAULT_GOAL := all
.PHONY: all build test docs run gen help

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
	$(GO) test ./...

docs:
	@echo "💬 Creating documentation..."
	@set -e; packages="$$($(GO) list ./...)"; \
	for package in $$packages; do \
		printf '\n=== %s ===\n' "$$package"; \
		$(GO) doc "$$package"; \
	done

run:
	@echo "🚀 Running the application..."
	$(GO) run ./cmd/restic-studio $(ARGS)

# === Capture ===
gen:
	@echo "🎧 Generating screenshots and videos with asciinema and ffmpeg"
	@bash docs/capture.sh

help:
	@echo "🛠️  Usage: make [target]"
	@echo ""
	@echo "Build:"
	@echo "  all        → 🎯 Run full test, docs, gen and build"
	@echo "  build      → 🏗️  Build the restic-studio executable"
	@echo ""
	@echo "Development:"
	@echo "  test       → 🔧 Run all Go tests"
	@echo "  docs       → 💬 Print Go package documentation"
	@echo "  run        → 🚀 Run the app (ARGS='--config /path/to/autorestic.yml')"
	@echo ""
	@echo "Generate Screenshots and Videos:"
	@echo "  gen        → 🎧 Generate screenshots and videos with asciinema and ffmpeg"
