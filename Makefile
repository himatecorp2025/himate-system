.PHONY: test-backend run-backend check

test-backend:
	cd services && go test ./...

run-backend:
	cd services && go run ./cmd/gateway

check: test-backend
	@echo "Go checks passed. Flutter SDK is required separately for flutter analyze/build."
