# syntax=docker/dockerfile:1.6

FROM golang:1.22-bookworm AS build

ARG SMARTCTL_EXPORTER_VERSION=v0.14.0
ARG GIT_COMMIT=unknown
ARG GOARCH=amd64

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN CGO_ENABLED=0 GOOS=windows GOARCH=${GOARCH} go build -trimpath \
    -ldflags="-s -w -X main.version=${SMARTCTL_EXPORTER_VERSION} -X main.commit=${GIT_COMMIT}" \
    -o /out/smartctl-exporter.exe ./cmd/smartctl-exporter

FROM scratch AS artifact
COPY --from=build /out/smartctl-exporter.exe /smartctl-exporter.exe
