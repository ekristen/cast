# syntax=docker/dockerfile:1.26-labs@sha256:63e440b412b6acba117974e793b7e7f702e58ee65e044bdff1b8d388ee0d853b
FROM cgr.dev/chainguard/wolfi-base:latest@sha256:03c6561658909fc4eadd0b2dc717375df40a22cc05455b8f82f1f1974e7e4427 AS base
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

FROM ubuntu:24.04
ARG PROJECT_NAME=cast
RUN apt-get update && apt-get install -y ca-certificates
COPY --from=build /src/bin/${PROJECT_NAME} /usr/local/bin/${PROJECT_NAME}
