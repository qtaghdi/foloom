GO ?= go
GO_TOOLCHAIN ?= go1.25.0
GO_CACHE ?= /tmp/document-workspace-go-cache
XLSX_PERFORMANCE_PROFILE ?= ci

.PHONY: audit build compatibility-test dev docx-compatibility-test docx-verify go-build go-test package-check package-smoke package-test package-verify release-build release-package-test release-verify test verify web-build web-install web-test xlsx-performance-verify xlsx-pre-excel-verify xlsx-safety-verify

VERSION ?= 0.1.0-alpha.local
HOST_OS := $(shell $(GO) env GOOS)
HOST_ARCH := $(shell $(GO) env GOARCH)

web-install:
	CI=true pnpm install --frozen-lockfile

web-test:
	CI=true pnpm --dir apps/web-editor run test
	CI=true pnpm --dir apps/web-editor run check:oss
	CI=true pnpm --dir apps/web-editor run typecheck

web-build:
	CI=true pnpm --dir apps/web-editor run build

# DOCX edits still compile both embedded editors. Only DOCX Go tests execute here.
docx-verify: web-build
	CI=true pnpm --dir apps/web-editor exec node --test tests/docx.test.mjs
	CI=true pnpm --dir apps/web-editor exec vitest run
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test ./internal/formats/docx
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test -race ./internal/formats/docx
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test ./internal/transport/httpapi -run DOCX -count=1
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test -race ./internal/transport/httpapi -run DOCX -count=1
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) vet ./internal/formats/docx ./internal/transport/httpapi

docx-compatibility-test:
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test ./internal/formats/docx -run Compatibility -count=1

go-test:
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test ./...

compatibility-test:
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test ./internal/formats/xlsx -run CompatibilityCorpus -count=1

go-build:
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) build -o bin/document-workspace ./cmd/document-workspace

package-check:
	node scripts/check-embedded-assets.mjs
	node scripts/check-private-alpha-policy.mjs

package-test:
	node --test scripts/check-embedded-assets.test.mjs scripts/check-private-alpha-policy.test.mjs

package-smoke: package-check go-build
	sh scripts/package-smoke.sh
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test ./internal/transport/httpapi -run 'TestEmbeddedUIRequiresSessionAndServesAssets|TestMCPAppToolAndResource' -count=1

package-verify: web-install web-build package-test package-smoke

release-build: web-build
	GO_TOOLCHAIN=$(GO_TOOLCHAIN) GO_CACHE=$(GO_CACHE) sh scripts/build-release.sh "$(VERSION)"

release-verify: release-build
	sh scripts/check-release-artifacts.sh

release-package-test: web-build
	GO_TOOLCHAIN=$(GO_TOOLCHAIN) GO_CACHE=$(GO_CACHE) RELEASE_TARGETS=$(HOST_OS)_$(HOST_ARCH) sh scripts/build-release.sh "$(VERSION)"
	RELEASE_TARGETS=$(HOST_OS)_$(HOST_ARCH) sh scripts/check-release-artifacts.sh
	sh scripts/release-smoke.sh "$(VERSION)"

xlsx-safety-verify: web-install
	REPORT="$(REPORT)" sh scripts/verify-xlsx-safety.sh

xlsx-performance-verify:
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) run ./cmd/xlsx-performance -profile "$(XLSX_PERFORMANCE_PROFILE)" -report "$(REPORT)"

xlsx-pre-excel-verify:
	REPORT="$(REPORT)" EVIDENCE_DIR="$(EVIDENCE_DIR)" sh scripts/verify-xlsx-pre-excel.sh

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
