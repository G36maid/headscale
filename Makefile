# Calculate version
version ?= $(shell git describe --always --tags --dirty)

rwildcard=$(foreach d,$(wildcard $1*),$(call rwildcard,$d/,$2) $(filter $(subst *,%,$2),$d))

# Determine if OS supports pie
GOOS ?= $(shell uname | tr '[:upper:]' '[:lower:]')
ifeq ($(filter $(GOOS), openbsd netbsd soloaris plan9), )
	pieflags = -buildmode=pie
else
endif

GO_SOURCES = $(call rwildcard,,*.go)
PROTO_SOURCES = $(call rwildcard,proto/,*.proto) # Scoping to proto directory
DOC_SOURCES = $(call rwildcard,,*.{ts,js,md,yaml,yml,sass,css,scss,html}) CHANGELOG.md

build:
	nix build

dev: lint test build

test:
	gotestsum -- -short -coverprofile=coverage.out ./...

test_integration:
	docker run \
		-t --rm \
		-v ~/.cache/hs-integration-go:/go \
		--name headscale-test-suite \
		-v $$PWD:$$PWD -w $$PWD/integration \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $$PWD/control_logs:/tmp/control \
		golang:1 \
		go run gotest.tools/gotestsum@latest -- -failfast ./... -timeout 120m -parallel 8

lint: lint-go lint-proto

lint-go: $(GO_SOURCES) go.mod go.sum
	@echo "Linting Go code..."
	golangci-lint run --timeout 10m

lint-proto: $(PROTO_SOURCES)
	@echo "Linting Protocol Buffer files..."
	cd proto/ && buf lint

fmt: fmt-go fmt-prettier fmt-proto

fmt-go: $(GO_SOURCES)
	@echo "Formatting Go code..."
	gofumpt -l -w .
	golines --max-len=88 --base-formatter=gofumpt -w $(GO_SOURCES)
	golangci-lint run --fix

fmt-prettier: $(DOC_SOURCES)
	@echo "Formatting documentation and config files..."
	prettier --write '**/*.{ts,js,md,yaml,yml,sass,css,scss,html}'
	prettier --write --print-width 80 --prose-wrap always CHANGELOG.md

fmt-proto: $(PROTO_SOURCES)
	@echo "Formatting Protocol Buffer files..."
	clang-format -style="{BasedOnStyle: Google, IndentWidth: 4, AlignConsecutiveDeclarations: true, AlignConsecutiveAssignments: true, ColumnLimit: 0}" -i $(PROTO_SOURCES)

compress: build
	upx --brute headscale

generate:
	rm -rf gen
	buf generate proto
