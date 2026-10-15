# Build stage: cross-compile on the build machine's own architecture, so
# multi-arch images don't need emulation.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o /out/rabbit ./cmd/rabbit

# Final image: just the static binary.
FROM scratch
COPY --from=build /out/rabbit /rabbit
USER 65534:65534
ENTRYPOINT ["/rabbit"]
