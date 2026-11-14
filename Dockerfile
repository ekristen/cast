# syntax=docker/dockerfile:1.29-labs@sha256:36366b12c1dffb228bb25e9feffb73707b93388f084f4927d2b7686dba29e76a
FROM cgr.dev/chainguard/wolfi-base:latest@sha256:1f6b749929164c3d9e7cad0289254f39b2ba51e1dddaeb0fa57532ea03f2db31 AS base
ARG PROJECT_NAME=cast
RUN apk add --no-cache ca-certificates
RUN addgroup -S ${PROJECT_NAME} && adduser -S ${PROJECT_NAME} -G ${PROJECT_NAME}

FROM golang:1.27 AS build
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

FROM ubuntu:26.04
ARG PROJECT_NAME=cast
RUN apt-get update && apt-get install -y ca-certificates
COPY --from=build /src/bin/${PROJECT_NAME} /usr/local/bin/${PROJECT_NAME}
