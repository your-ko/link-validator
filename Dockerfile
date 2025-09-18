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

#FROM alpine:3.20
FROM scratch

COPY --from=builder /etc/passwd_gouser /etc/passwd
USER gouser
#
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /src/bin/link-validator /app/link-validator
ENTRYPOINT ["/app/link-validator"]
#ENTRYPOINT ["/bin/sh"]
#CMD []
