# Build every operations tool without requiring Go on the host.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY scripts/ops /src/ops
COPY scripts/quota /src/quota
COPY scripts/wssmoke /src/wssmoke
COPY scripts/bench /src/bench
COPY scripts/tools /src/tools
COPY scripts/public /src/public
COPY scripts/hara /src/hara
RUN mkdir /bin-tools && for tool in ops quota wssmoke bench; do \
    cd /src/$tool && CGO_ENABLED=0 go build -trimpath -tags timetzdata -o /bin-tools/$tool . || exit 1; \
    done
RUN cd /src/tools && CGO_ENABLED=0 go build -trimpath -tags timetzdata -o /bin-tools/hara-tools .

# Development verification also runs with the image's Go toolchain.
FROM build AS verify
RUN apk add --no-cache git
RUN for tool in ops quota wssmoke bench tools public hara; do \
    cd /src/$tool && go vet ./... && go test ./... || exit 1; \
    done

FROM alpine:3.23
RUN apk add --no-cache ca-certificates
COPY --from=build /bin-tools /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/hara-tools"]
