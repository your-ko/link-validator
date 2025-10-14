FROM golang:1.24.2-alpine3.20 AS builder

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
RUN GOOS=linux GOARCH=amd64 make build

FROM scratch

ARG BUILD_DATE
ARG GIT_COMMIT
ARG VERSION

LABEL org.opencontainers.image.title="Link Validator" \
      org.opencontainers.image.description="A simple link validator for markdown and code repositories" \
      org.opencontainers.image.url="https://github.com/your-ko/link-validator" \
      org.opencontainers.image.source="https://github.com/your-ko/link-validator" \
      org.opencontainers.image.version="$VERSION" \
      org.opencontainers.image.created=$BUILD_DATE \
      org.opencontainers.image.revision=$GIT_COMMIT \
      org.opencontainers.image.licenses="MIT"

COPY --from=builder /etc/passwd_gouser /etc/passwd
USER gouser

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /src/bin/link-validator /app/link-validator
ENTRYPOINT ["/app/link-validator"]
