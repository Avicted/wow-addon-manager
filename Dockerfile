# syntax=docker/dockerfile:1

# wam: one static Go binary. It only needs the network and the mounted AddOns
# directory, so the runtime image is distroless (CA certificates, nothing else).
#
#   make image                   build this image (tests run as part of the build)
#   make dist                    export the static binary to build/dist/wam
#   make status / update / ...   run it against your AddOns directory

# Images are pinned by digest so a re-pushed tag cannot change what builds;
# Dependabot (docker ecosystem) proposes the updates.
FROM golang:1.27-alpine@sha256:eb61c3873c97416fff0116553fff65d67a3549f72e11d9758849a689cdd8b523 AS source
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
WORKDIR /src
COPY go.mod ./
COPY cmd/ cmd/
COPY internal/ internal/

# A failing vet or test fails every image and binary build: compile depends on this stage.
# The Windows vet keeps the per-OS files (lock, process check, paths) compiling there.
FROM source AS test
RUN --mount=type=cache,target=/root/.cache/go-build \
    go vet ./... && GOOS=windows go vet ./... && go test ./... && touch /tested

# The race detector needs cgo, so a C toolchain; the shipped binary stays CGO-free.
# `make test` runs this stage; image builds don't wait for it.
FROM source AS race
# hadolint ignore=DL3018
RUN apk add --no-cache gcc musl-dev
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go test -race ./...

# `make cover` exports coverage.out, a per-function summary and an HTML report.
FROM source AS cover
RUN --mount=type=cache,target=/root/.cache/go-build \
    mkdir /cover && go test -covermode=atomic -coverprofile=/cover/coverage.out ./... \
    && go tool cover -func=/cover/coverage.out > /cover/coverage.txt \
    && go tool cover -html=/cover/coverage.out -o /cover/coverage.html

FROM scratch AS coverage
COPY --from=cover /cover/ /

FROM source AS compile
ARG VERSION=dev
COPY --from=test /tested /tested
RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/wam ./cmd/wam \
    && GOOS=windows go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/wam.exe ./cmd/wam \
    && /out/wam --version

# `docker build --target binary --output build/dist .` exports the Linux and Windows binaries.
FROM scratch AS binary
COPY --from=compile /out/wam /out/wam.exe /

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
LABEL org.opencontainers.image.source="https://github.com/Avicted/wow-addon-manager" \
      org.opencontainers.image.description="wam: WoW Forever addon manager"
COPY --from=compile /out/wam /usr/local/bin/wam
# The Makefile runs it as your own user so files written to AddOns stay yours;
# without that it is distroless's unprivileged "nonroot" user (65532).
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/wam"]
