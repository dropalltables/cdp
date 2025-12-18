.PHONY: build install clean test help

BINARY_NAME=cdp
BUILD_DIR=bin

help:
	@echo "Available targets:"
	@echo "  build    - Build the binary to ./$(BUILD_DIR)/$(BINARY_NAME)"
	@echo "  install  - Install the binary to $$GOPATH/bin"
	@echo "  clean    - Remove build artifacts"
	@echo "  test     - Run tests"

build:
	@mkdir -p $(BUILD_DIR)
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

install:
	@echo "Installing $(BINARY_NAME)..."
	@go install .
	@echo "Installed to $$GOPATH/bin/$(BINARY_NAME)"

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)
	@go clean
	@echo "Clean complete"

test:
	@echo "Running tests..."
	@go test -v ./...
