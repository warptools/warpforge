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
	$(SERUM) ./...
endif
	go test ./...

all: test install

.PHONY: install test all
