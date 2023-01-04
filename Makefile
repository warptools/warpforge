GOBIN = $(shell go env GOPATH)/bin
SERUM := $(shell command -v go-serum-analyzer)
MODULE := $(shell go list -m)
WARPLARK := $(shell command -v warplark)

install: warplark
	@echo "Installing plugins..."
	cp ./plugins/* $(GOBIN)
	@echo "Building and installing warpforge..."
	go install ./...
	@echo "Install complete!"

test: warplark
ifndef SERUM
	@echo "go-serum-analyzer executable not found, skipping error analysis"
	@echo "go-serum-analyzer can be installed from https://github.com/serum-errors/go-serum-analyzer"
	@echo
else
	$(SERUM) -strict ./...
endif
	go test ./...
	@stty sane

warplark: check-warplark
ifndef WARPLARK
	@echo "Building and installing warplark..."
	go install github.com/warptools/warplark
endif

update-warplark:
	go install github.com/warptools/warplark@latest
	sha256sum $$(which warplark) | cut -f1 -d' ' > tools/warplark.sha256

# janky way of validating warplark being the correct version
check-warplark:
ifndef WARPLARK
	@echo "warplark not found"
else
	@cat tools/warplark.sha256 tools/warplark.sha256.tmp | tr -d '\n' > tools/warplark.sha256.tmp
	@echo "  $$(which warplark)" >> tools/warplark.sha256.tmp
	- @sha256sum --check tools/warplark.sha256.tmp
endif

imports:
	goimports -local=$(MODULE) -w ./cmd ./pkg ./wfapi ./larkdemo

imports-test:
# ideally this would return a non-zero exit code when it detects output
	goimports -local=$(MODULE) -l ./cmd ./pkg ./wfapi ./larkdemo

vet:
	go vet ./...

shadow:
# go install golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow@latest
	@command -v shadow # errors if shadow is not an available command
	go vet -vettool=$$(command -v shadow) ./...

all: test install

.PHONY: install test all shadow vet imports imports-test check-warplark update-warplark warplark

