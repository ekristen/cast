# syntax=docker/dockerfile:1.14-labs@sha256:bbd3af5735ce9921cfa2a8b59d1006445333e161cb4582c3d88aad0730d9b207
FROM cgr.dev/chainguard/wolfi-base:latest@sha256:f57d9086dc184b79093d4389ed8e31ca64102d04e4551103b07a7f6b78072e3b AS base
ARG PROJECT_NAME=cast
RUN apk add --no-cache ca-certificates
RUN addgroup -S ${PROJECT_NAME} && adduser -S ${PROJECT_NAME} -G ${PROJECT_NAME}

FROM golang:1.22 AS build
ARG PROJECT_NAME=cast
COPY / /src
WORKDIR /src
RUN \
  --mount=type=cache,target=/go/pkg \
  --mount=type=cache,target=/root/.cache/go-build \
  go build -o bin/${PROJECT_NAME} main.go

FROM base AS goreleaser
ARG PROJECT_NAME=cast
COPY ${PROJECT_NAME} /usr/local/bin/${PROJECT_NAME}
USER ${PROJECT_NAME}

FROM ubuntu:22.04
ARG PROJECT_NAME=cast
RUN apt-get update && apt-get install -y ca-certificates
COPY --from=build /src/bin/${PROJECT_NAME} /usr/local/bin/${PROJECT_NAME}
