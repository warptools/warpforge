GOBIN = $(shell go env GOPATH)/bin
SERUM := $(shell command -v go-serum-analyzer)
MODULE := $(shell go list -m)

install:
	@echo "Installing plugins..."
	mkdir -p $(GOBIN)
	cp ./plugins/* $(GOBIN)
	@echo "Building and installing warpforge..."
	go install ./...
	@echo "Install complete!"

test:
ifndef SERUM
	@echo "go-serum-analyzer executable not found, skipping error analysis"
	@echo "go-serum-analyzer can be installed from https://github.com/serum-errors/go-serum-analyzer"
	@echo
else
	-$(SERUM) -strict ./...
endif
	go test ./...
	@stty sane

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

.PHONY: install test all shadow vet imports imports-test

