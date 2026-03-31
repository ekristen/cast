# syntax=docker/dockerfile:1.23-labs@sha256:7eca9451d94f9b8ad22e44988b92d595d3e4d65163794237949a8c3413fbed5d
FROM cgr.dev/chainguard/wolfi-base:latest@sha256:a5a619c1793039dcf92f02178f37c94bb3d6001403716da59d6092dfe8d9b502 AS base
ARG PROJECT_NAME=cast
RUN apk add --no-cache ca-certificates
RUN addgroup -S ${PROJECT_NAME} && adduser -S ${PROJECT_NAME} -G ${PROJECT_NAME}

FROM golang:1.26 AS build
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
