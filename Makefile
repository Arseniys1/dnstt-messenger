.PHONY: validate-translations install-deps help

# Default target
help:
	@echo "DNSTT Messenger - Available Commands"
	@echo ""
	@echo "  make validate-translations  - Validate translation file completeness"
	@echo "  make install-deps          - Install dependencies for validation script"
	@echo "  make help                  - Show this help message"
	@echo ""

# Install dependencies needed for the validation script
install-deps:
	@echo "Installing dependencies for translation validation..."
	cd electron-client && npm install
	@echo "Dependencies installed successfully!"

# Validate all translation files
validate-translations:
	@echo "Running translation validation..."
	@node scripts/validate-translations.js
