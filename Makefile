# Owlsort CLI. No cgo anywhere: every platform builds from one machine.
export CGO_ENABLED = 0

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build test vet fmt check install-test cross snapshot release-check clean

build:            ## build ./owlsort for this machine
	go build -ldflags '$(LDFLAGS)' -o owlsort ./cmd/owlsort

test:             ## run the tests
	go test ./...

vet:
	go vet ./...

fmt:              ## format the code
	gofmt -w .

check: vet test   ## what CI runs, plus formatting
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)

install-test:     ## test install.sh against a fake release, on this machine
	sh scripts/test-install.sh

cross:            ## build every platform, to prove they compile (output in dist/)
	@mkdir -p dist
	@for t in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do \
		os=$${t%/*}; arch=$${t#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		GOOS=$$os GOARCH=$$arch go build -ldflags '$(LDFLAGS)' -o dist/owlsort-$$os-$$arch$$ext ./cmd/owlsort || exit 1; \
		echo "built dist/owlsort-$$os-$$arch$$ext"; \
	done

release-check:    ## check .goreleaser.yaml and build a snapshot; what CI runs
	goreleaser check
	goreleaser release --snapshot --clean

snapshot:         ## a local, unsigned release build (needs goreleaser)
	goreleaser release --snapshot --clean

clean:
	rm -rf dist owlsort owlsort.exe
