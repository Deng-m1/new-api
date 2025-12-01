FRONTEND_DIR = ./web
BACKEND_DIR = .

.PHONY: all build-frontend start-backend lint lint-fix

all: build-frontend start-backend

build-frontend:
	@echo "Building frontend..."
	@cd $(FRONTEND_DIR) && bun install && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat VERSION) bun run build

start-backend:
	@echo "Starting backend dev server..."
	@cd $(BACKEND_DIR) && go run main.go &

lint:
	@echo "Running Go linter..."
	@.\golangci-lint.exe run ./...

lint-fix:
	@echo "Running Go linter with auto-fix..."
	@.\golangci-lint.exe run --fix ./...
