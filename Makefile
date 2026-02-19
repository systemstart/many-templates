COVERAGE_THRESHOLD := 80
COVERPROFILE := coverage.out

.PHONY: build test check release clean

build:
	CGO_ENABLED=1 go build -o many ./cmd/many

test:
	@echo "Running tests (coverage threshold: $(COVERAGE_THRESHOLD)%)..."
	CGO_ENABLED=1 go test -race -coverprofile=$(COVERPROFILE) -covermode=atomic ./pkg/...
	@total=$$(go tool cover -func=$(COVERPROFILE) | awk '/^total:/ {gsub(/%/,"",$$NF); print $$NF}'); \
	printf "Total coverage: %s%%\n" "$$total"; \
	if awk "BEGIN{exit !($$total < $(COVERAGE_THRESHOLD))}"; then \
		echo "FAIL: coverage $$total%% is below $(COVERAGE_THRESHOLD)%%"; \
		exit 1; \
	fi

check:
	golangci-lint run

release:
	goreleaser release --clean

clean:
	rm -f $(COVERPROFILE)
