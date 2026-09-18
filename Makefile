# Local developer targets. `make ci` mirrors what the CI workflow runs, so a
# clean local run is a good predictor of a green pipeline.

.PHONY: all ci fmt fmtcheck vet build test race cover fuzz examples hooks tidy

all: ci

# ci runs the same gates as .github/workflows/ci.yml
ci: fmtcheck vet build race

fmt:
	gofmt -w .

fmtcheck:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "not gofmt-clean:"; echo "$$unformatted"; exit 1; \
	fi

vet:
	go vet ./...

build:
	go build ./...

test:
	go test ./...

race:
	go test -race ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

# Short fuzz smoke run over the risky input boundaries
fuzz:
	go test ./path/ -run='^$$' -fuzz=FuzzRasterize -fuzztime=30s
	go test ./path/ -run='^$$' -fuzz=FuzzStroke -fuzztime=30s
	go test ./gradient/ -run='^$$' -fuzz=FuzzGradient -fuzztime=30s

# Regenerate the example images
examples:
	@for d in blendmodes gradients effects paths showcase banner; do go run ./examples/$$d; done

# Enable the repo's git hooks (runs fmt, vet, and tests before each commit)
hooks:
	git config core.hooksPath .githooks
	@echo "git hooks enabled from .githooks"

tidy:
	go mod tidy
