.PHONY: test-backend run-backend check

test-backend:
	cd backend && go test ./...

run-backend:
	cd backend && go run ./cmd/api

check: test-backend
	@echo "Go checks passed. Flutter SDK is required separately for flutter analyze/build."
