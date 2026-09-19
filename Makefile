.PHONY: web build linux test
web:
	cd web && npm ci && npm run build
build: web
	go build -trimpath -o bin/fabricworld ./cmd/fabricworld
linux: web
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o bin/fabricworld-linux-amd64 ./cmd/fabricworld
test:
	go test ./...
	go vet ./...
	cd web && npm test
