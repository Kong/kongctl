.PHONY: test-all
test-all: lint test test-integration

VERSION ?= $(shell (git describe --tags --exact-match 2>/dev/null || echo dev) | sed 's/^v//')
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(GIT_COMMIT) -X main.date=$(BUILD_DATE)
LATEST_E2E_LINK ?= .latest-e2e
LATEST_BENCHMARK_LINK ?= .latest-benchmark

.PHONY: lint
lint:
	golangci-lint run -v

.PHONY: format fmt
format:
	gofumpt -l -w . 
	golines -m 120 -w --base-formatter=gofumpt .
fmt: format

.PHONY: mod
mod:
	go mod tidy
	go mod vendor

.PHONY: build
build: mod
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o kongctl

.PHONY: build-ci
build-ci:
	CGO_ENABLED=0 go build -mod=readonly -ldflags "$(LDFLAGS)" -o kongctl
# Kept typing this wrong
buld: build

.PHONY: build-docker
build-docker:
	@set -eu; \
	mkdir -p linux/amd64; \
	rm -rf linux/amd64/kongctl; \
	trap 'rm -rf linux/amd64/kongctl' 0; \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o linux/amd64/kongctl .; \
	docker buildx build --platform linux/amd64 --load -t kongctl:$(VERSION) .

.PHONY: coverage
coverage:
	go test -race -v -count=1 -coverprofile=coverage.out.tmp ./...
	# ignoring generated code for coverage
	grep -E -v 'generated.deepcopy.go' coverage.out.tmp > coverage.out
	rm -f coverage.out.tmp

.PHONY: test
test:
	go test -race -count=1 ./...

.PHONY: test-integration
test-integration:
	go test -v -count=1 -tags=integration \
		-race \
		${GOTESTFLAGS} \
		./test/integration/...

.PHONY: test-e2e
test-e2e:
	@ART_DIR="$$KONGCTL_E2E_ARTIFACTS_DIR"; \
	if [ -z "$$ART_DIR" ]; then \
		ART_DIR=$$(mktemp -d 2>/dev/null || mktemp -d -t kongctl-e2e || echo .e2e_artifacts); \
	else \
		mkdir -p "$$ART_DIR"; \
		run_id=$$(date +%Y%m%d-%H%M%S); \
		ART_DIR="$$ART_DIR/$$run_id"; \
	fi; \
	mkdir -p "$$ART_DIR"; \
	ART_DIR=$$(cd "$$ART_DIR" && pwd); \
	( KONGCTL_E2E_ARTIFACTS_DIR="$$ART_DIR" go test -v -count=1 -tags=e2e $${GOTESTFLAGS} ./test/e2e/... ; echo $$? > "$$ART_DIR/.exit_code" ) | tee "$$ART_DIR/run.log"; \
	code=$$(cat "$$ART_DIR/.exit_code"); rm -f "$$ART_DIR/.exit_code"; \
	echo "E2E artifacts: $$ART_DIR"; \
	ln -sfn "$$ART_DIR" "$(LATEST_E2E_LINK)" || true; \
	exit $$code

.PHONY: test-e2e-scenarios
test-e2e-scenarios:
	@ART_DIR="$$KONGCTL_E2E_ARTIFACTS_DIR"; \
	if [ -z "$$ART_DIR" ]; then \
		ART_DIR=$$(mktemp -d 2>/dev/null || mktemp -d -t kongctl-e2e || echo .e2e_artifacts); \
	else \
		mkdir -p "$$ART_DIR"; \
		run_id=$$(date +%Y%m%d-%H%M%S); \
		ART_DIR="$$ART_DIR/$$run_id"; \
	fi; \
	mkdir -p "$$ART_DIR"; \
	ART_DIR=$$(cd "$$ART_DIR" && pwd); \
	( KONGCTL_E2E_ARTIFACTS_DIR="$$ART_DIR" \
	  KONGCTL_E2E_SCENARIO="${SCENARIO}" \
	  KONGCTL_E2E_STOP_AFTER="${STOP_AFTER}" \
	  go test -v -count=1 -tags=e2e -run '^Test_Scenarios$$' $${GOTESTFLAGS} ./test/e2e ; \
	  echo $$? > "$$ART_DIR/.exit_code" ) | tee "$$ART_DIR/run.log"; \
	code=$$(cat "$$ART_DIR/.exit_code"); rm -f "$$ART_DIR/.exit_code"; \
	echo "E2E artifacts: $$ART_DIR"; \
	ln -sfn "$$ART_DIR" "$(LATEST_E2E_LINK)" || true; \
	exit $$code

.PHONY: scenario
scenario: test-e2e-scenarios

.PHONY: benchmark-declarative
benchmark-declarative:
	@set -eu; \
	ART_DIR="$${KONGCTL_BENCHMARK_ARTIFACTS_DIR:-}"; \
	if [ -z "$$ART_DIR" ]; then \
		ART_DIR=".benchmark-artifacts"; \
	fi; \
	mkdir -p "$$ART_DIR"; \
	run_id=$$(date +%Y%m%d-%H%M%S); \
	ART_DIR="$$ART_DIR/$$run_id"; \
	mkdir -p "$$ART_DIR"; \
	ART_DIR=$$(cd "$$ART_DIR" && pwd); \
	( KONGCTL_BENCHMARK_ARTIFACTS_DIR="$$ART_DIR" \
	  KONGCTL_E2E_ARTIFACTS_DIR="$$ART_DIR" \
	  code=0; \
	  go run -tags=e2e ./test/benchmarks/declarative $(BENCHMARK_FLAGS) || code=$$?; \
	  echo $$code > "$$ART_DIR/.exit_code"; \
	  exit $$code ) | tee "$$ART_DIR/run.log"; \
	code=$$(cat "$$ART_DIR/.exit_code"); rm -f "$$ART_DIR/.exit_code"; \
	if [ -f "$$ART_DIR/summary.txt" ]; then \
		echo; \
		cat "$$ART_DIR/summary.txt"; \
		echo; \
	elif [ -f "$$ART_DIR/summary.md" ]; then \
		echo; \
		cat "$$ART_DIR/summary.md"; \
		echo; \
	fi; \
	echo "Declarative benchmark artifacts: $$ART_DIR"; \
	ln -sfn "$$ART_DIR" "$(LATEST_BENCHMARK_LINK)" || true; \
	exit $$code

.PHONY: benchmark-declarative-case
benchmark-declarative-case:
	@if [ -z "$(CASE)" ]; then \
		echo "CASE is required, for example: make benchmark-declarative-case CASE=medium-single" >&2; \
		exit 1; \
	fi
	@$(MAKE) benchmark-declarative BENCHMARK_FLAGS="--case $(CASE) $(BENCHMARK_FLAGS)"

.PHONY: open-latest-e2e
open-latest-e2e:
	@set -euo pipefail; \
	if [ -L "$(LATEST_E2E_LINK)" ]; then \
		readlink -f "$(LATEST_E2E_LINK)"; \
		exit 0; \
	fi; \
	if [ -f .e2e_artifacts_dir ]; then \
		awk 'NR==1 {print $$0}' .e2e_artifacts_dir; \
		exit 0; \
	fi; \
		echo "no latest e2e artifacts found" >&2; \
		exit 1

.PHONY: analyze-latest-e2e
analyze-latest-e2e:
	@set -euo pipefail; \
	ART_DIR=$$($(MAKE) -s open-latest-e2e 2>/dev/null) || { echo "latest e2e artifacts not found; run tests first" >&2; exit 1; }; \
	if [ ! -d "$$ART_DIR" ]; then \
		echo "artifacts dir $$ART_DIR not found" >&2; exit 1; \
	fi; \
	PROMPT_FILE="test/e2e/e2e_analysis_prompt.txt"; \
	if [ ! -f "$$PROMPT_FILE" ]; then \
		echo "prompt file $$PROMPT_FILE missing" >&2; exit 1; \
	fi; \
	PROMPT=$$(printf "%s\n\nLatest artifacts: %s (also via .latest-e2e)." "$$(cat "$$PROMPT_FILE")" "$$ART_DIR"); \
	LATEST_E2E_DIR="$$ART_DIR" codex exec --sandbox read-only --cd "$(CURDIR)" "$$PROMPT"

.PHONY: reset-org
reset-org:
	@echo "Resetting Konnect org (requires KONGCTL_E2E_KONNECT_PAT, optional KONGCTL_E2E_KONNECT_BASE_URL, KONGCTL_E2E_RESET)"
	go run -tags e2e ./test/e2e/harness/cmd/reset-org --stage make-reset
