# Build every operations tool without requiring Go on the host.
FROM golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
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

FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
RUN apk add --no-cache ca-certificates
COPY --from=build /bin-tools /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/hara-tools"]
