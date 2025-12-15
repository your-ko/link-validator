# renovate: datasource=github-releases depName=vektra/mockery versioning=semver
MOCKERY_VERSION=v3.6.1

download:
	go mod download

tidy:
	go mod tidy

test: download
	CGO_ENABLED=1 go test -race ./cmd/... ./internal/... ./pkg/...

lint:
	go vet ./cmd/... ./internal/... ./pkg/...

fmt:
	go fmt ./cmd/... ./internal/... ./pkg/...

build: download
	mkdir -p bin
	go build -o bin/link-validator ./cmd/link-validator/main.go

docker-build:
	docker build . -t link-validator

generate-mocks:
	docker run --rm -v "$$PWD:/src" -w /src/pkg/github vektra/mockery:${MOCKERY_VERSION}
	docker run --rm -v "$$PWD:/src" -w /src/pkg/dd vektra/mockery:${MOCKERY_VERSION}
