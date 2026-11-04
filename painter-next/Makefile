.PHONY: build frontend test check run
frontend:
	cd web/frontend && npm ci && npm run build
build:
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/painter ./cmd/painter
test:
	go test -race ./...
check:
	go vet ./...
	cd web/frontend && npm run build
run:
	go run ./cmd/painter serve
