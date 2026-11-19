# The quota service in local/compose.yaml: `quota -route -every 1h` next to the proxy.
FROM golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
WORKDIR /src
COPY go.mod *.go ./
# timetzdata embeds the zone database: the final image has no /usr/share/zoneinfo for TZ.
RUN CGO_ENABLED=0 go build -trimpath -tags timetzdata -o /quota .

FROM scratch
COPY --from=build /quota /quota
ENTRYPOINT ["/quota"]
