GOBIN=$(shell go env GOPATH)/bin

install:
	@echo "Installing plugins..."
	cp ./plugins/* $(GOBIN)
	@echo
	@echo "Building and installing warpforge..."
	go install ./...
	@echo
	@echo "Install complete!"

test:
	go test ./...