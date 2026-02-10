# Codes Makefile
.PHONY: build clean test version help install

BINARY = codes

help:
	@echo "Available targets:"
	@echo "  build    - Build codes"
	@echo "  clean    - Clean build artifacts"
	@echo "  test     - Test the build"
	@echo "  version  - Show version"
	@echo "  install  - Build and run init (install + setup)"
	@echo ""
	@echo "Usage: make [target]"

build:
	@echo "Building codes..."
	@go build -o $(BINARY) ./cmd/codes
	@echo "Build completed"

clean:
	@echo "Cleaning..."
	@rm -f $(BINARY)
	@echo "Clean completed"

test:
	@echo "Testing..."
	@if [ -f "$(BINARY)" ]; then \
		./$(BINARY) --help > /dev/null 2>&1 && echo "✓ Help works"; \
		./$(BINARY) version && echo "✓ Version works"; \
		./$(BINARY) completion bash > /dev/null 2>&1 && echo "✓ Completion works"; \
	else \
		echo "✗ Not built"; \
	fi

version:
	@./$(BINARY) version 2>/dev/null || echo "Not built"

install: build
	@echo "Installing codes..."
	@./$(BINARY) init
	@echo "Install completed"