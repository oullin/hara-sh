# The quota service in local/compose.yaml: `quota -route -every 1h` next to the proxy.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod *.go ./
# timetzdata embeds the zone database: the final image has no /usr/share/zoneinfo for TZ.
RUN CGO_ENABLED=0 go build -trimpath -tags timetzdata -o /quota .

FROM scratch
COPY --from=build /quota /quota
ENTRYPOINT ["/quota"]
