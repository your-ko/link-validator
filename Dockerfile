FROM golang:1.24.3-alpine3.20 AS builder

ARG BUILD_DATE
ARG GIT_COMMIT
ARG VERSION

ARG CA_CERT_VERSION=20241121-r1
ARG MAKE_VERSION=4.4.1-r2
ARG BB_VERSION=0.5-r3

RUN apk update && apk add --no-cache \
    ca-certificates=${CA_CERT_VERSION} \
    make=${MAKE_VERSION} \
    build-base=${BB_VERSION} \
    && update-ca-certificates

RUN addgroup gouser &&\
     adduser --ingroup gouser --uid 2000 --disabled-password --shell /bin/false gouser && \
     cat /etc/passwd | grep gouser > /etc/passwd_gouser

COPY . /src
WORKDIR /src

ENV CGO_ENABLED=0
RUN GOOS=linux GOARCH=amd64 go build -ldflags \
    "-X main.GitCommit=$GIT_COMMIT -X main.BuildDate=$BUILD_DATE -X main.Version=$VERSION" \
    -o bin/link-validator ./cmd/link-validator/main.go

FROM scratch

ARG BUILD_DATE
ARG GIT_COMMIT
ARG VERSION

COPY --from=builder /etc/passwd_gouser /etc/passwd
USER gouser

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /src/bin/link-validator /app/link-validator
ENTRYPOINT ["/app/link-validator"]
