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
	go test -v -count=1 -tags=e2e ./test/e2e/...
	@if [ -n "$$KONGCTL_E2E_ARTIFACTS_DIR" ]; then \
		echo "E2E artifacts: $$KONGCTL_E2E_ARTIFACTS_DIR"; \
	elif [ -f .e2e_artifacts_dir ]; then \
		echo "E2E artifacts: $$(cat .e2e_artifacts_dir)"; \
		rm -f .e2e_artifacts_dir; \
	fi
