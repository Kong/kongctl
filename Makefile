.PHONY: test-all
test-all: lint test test-integration

.PHONY: lint
lint:
	golangci-lint run -v ./...

.PHONY: format fmt
format:
	gofumpt -l -w . 
	golines -m 120 -w --base-formatter=gofumpt .
fmt: format


.PHONY: build
build:
	CGO_ENABLED=0 go build -o kongctl
# Kept typing this wrong
buld: build

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
	fi; \
	mkdir -p "$$ART_DIR"; \
	( KONGCTL_E2E_ARTIFACTS_DIR="$$ART_DIR" go test -v -count=1 -tags=e2e $${GOTESTFLAGS} ./test/e2e/... ; echo $$? > "$$ART_DIR/.exit_code" ) | tee "$$ART_DIR/run.log"; \
	code=$$(cat "$$ART_DIR/.exit_code"); rm -f "$$ART_DIR/.exit_code"; \
	echo "E2E artifacts: $$ART_DIR"; \
	exit $$code

.PHONY: test-e2e-scenarios
test-e2e-scenarios:
	@ART_DIR="$$KONGCTL_E2E_ARTIFACTS_DIR"; \
	if [ -z "$$ART_DIR" ]; then \
		ART_DIR=$$(mktemp -d 2>/dev/null || mktemp -d -t kongctl-e2e || echo .e2e_artifacts); \
	fi; \
	mkdir -p "$$ART_DIR"; \
	( KONGCTL_E2E_ARTIFACTS_DIR="$$ART_DIR" \
	  KONGCTL_E2E_SCENARIO="${SCENARIO}" \
	  go test -v -count=1 -tags=e2e -run '^Test_Scenarios$$' $${GOTESTFLAGS} ./test/e2e ; \
	  echo $$? > "$$ART_DIR/.exit_code" ) | tee "$$ART_DIR/run.log"; \
	code=$$(cat "$$ART_DIR/.exit_code"); rm -f "$$ART_DIR/.exit_code"; \
	echo "E2E artifacts: $$ART_DIR"; \
	exit $$code
