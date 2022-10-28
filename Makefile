GOBIN = $(shell go env GOPATH)/bin
SERUM := $(shell command -v go-serum-analyzer)

install:
	@echo "Installing plugins..."
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
	$(SERUM) -strict ./...
endif
	go test ./...
	@stty sane

imports:
	goimports -w ./cmd
	goimports -w ./pkg
	goimports -w ./wfapi
	goimports -w ./larkdemo

vet:
	go vet ./...

shadow:
# go install golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow@latest
	@command -v shadow # errors if shadow is not an available command
	go vet -vettool=$$(command -v shadow) ./...

all: test install

.PHONY: install test all shadow vet imports
