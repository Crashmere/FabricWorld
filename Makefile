.PHONY: web build linux
web:
	cd web && npm run build
build: web
	go build -trimpath -o bin/fabricworld ./cmd/fabricworld
linux: web
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o bin/fabricworld-linux-amd64 ./cmd/fabricworld

.PHONY: release deploy portal rollback releases
release:
	bash deploy/release.sh build
deploy:
	bash deploy/release.sh deploy
portal:
	bash deploy/release.sh portal
rollback:
	bash deploy/release.sh rollback $(COMMIT)
releases:
	bash deploy/release.sh list
