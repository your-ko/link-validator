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
