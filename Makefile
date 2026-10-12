GO ?= go
GO_TOOLCHAIN ?= go1.25.0
GO_CACHE ?= /tmp/document-workspace-go-cache

.PHONY: audit build compatibility-test dev go-build go-test package-check package-smoke package-test package-verify test verify web-build web-install web-test xlsx-safety-verify

web-install:
	CI=true pnpm install --frozen-lockfile

web-test:
	CI=true pnpm --dir apps/web-editor run test
	CI=true pnpm --dir apps/web-editor run check:oss
	CI=true pnpm --dir apps/web-editor run typecheck

web-build:
	CI=true pnpm --dir apps/web-editor run build

go-test:
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test ./...

compatibility-test:
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test ./internal/formats/xlsx -run CompatibilityCorpus -count=1

go-build:
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) build -o bin/document-workspace ./cmd/document-workspace

package-check:
	node scripts/check-embedded-assets.mjs

package-test:
	node --test scripts/check-embedded-assets.test.mjs

package-smoke: package-check go-build
	sh scripts/package-smoke.sh
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test ./internal/transport/httpapi -run 'TestEmbeddedUIRequiresSessionAndServesAssets|TestMCPAppToolAndResource' -count=1

package-verify: web-install web-build package-test package-smoke

xlsx-safety-verify: web-install
	REPORT="$(REPORT)" sh scripts/verify-xlsx-safety.sh

test: web-test go-test

verify: web-build
	CI=true pnpm --dir apps/web-editor run test
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test ./...
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test -race ./...
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) vet ./...
	python3 legacy/xlsx-python/test_server.py

build: web-build package-check go-build

audit:
	pnpm audit --prod

dev:
	@test -n "$(FILE)" || (echo "Usage: make dev FILE=/absolute/path/to/book.xlsx" && exit 2)
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) run ./cmd/document-workspace -file "$(FILE)"
