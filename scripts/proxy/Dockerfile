FROM golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea AS build

WORKDIR /src
ENV GOTOOLCHAIN=local

# CLIProxyAPI v8.0.78, pinned to its release commit and the source archive's SHA-256.
ADD --checksum=sha256:072e3a8f73f5497f8a65a3cdc701554105ba6d195b6bcb7c59c8be25ec3a286f https://codeload.github.com/router-for-me/CLIProxyAPI/tar.gz/db7f177d6b0e51eb237daed718de4d25e7a893f5 /tmp/proxy.tar.gz
COPY dependencies.patch /tmp/dependencies.patch
# The archive checksum above fixes the exact input to the context-free module patch.
RUN tar -xzf /tmp/proxy.tar.gz --strip-components=1 && git apply --unidiff-zero /tmp/dependencies.patch
RUN go mod download && go mod verify
RUN CGO_ENABLED=1 go build -mod=readonly -buildvcs=false -trimpath \
    -ldflags="-s -w -X main.Version=v8.0.78+hara -X main.Commit=db7f177d6b0e51eb237daed718de4d25e7a893f5" \
    -o /tmp/CLIProxyAPI ./cmd/server

FROM debian:bookworm-slim@sha256:734a72b3d6142a71993b4bc580ca01d8ca687f77a2c422d96a961d3376b179bc

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /CLIProxyAPI
COPY --from=build /tmp/CLIProxyAPI ./
COPY --from=build /src/config.example.yaml /src/LICENSE ./
ENV TZ=UTC
EXPOSE 8317
CMD ["./CLIProxyAPI"]
