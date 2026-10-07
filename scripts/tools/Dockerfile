# Build every operations tool without requiring Go on the host.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY scripts/ops /src/ops
COPY scripts/quota /src/quota
COPY scripts/wssmoke /src/wssmoke
COPY scripts/bench /src/bench
RUN mkdir /bin-tools && for tool in ops quota wssmoke bench; do \
    cd /src/$tool && CGO_ENABLED=0 go build -trimpath -tags timetzdata -o /bin-tools/$tool . || exit 1; \
    done

# Development verification also runs with the image's Go toolchain.
FROM build AS verify
RUN for tool in ops quota wssmoke bench; do \
    cd /src/$tool && go vet ./... && go test ./... || exit 1; \
    done

FROM python:3.14-alpine
RUN pip install --no-cache-dir bcrypt==5.0.0
COPY --from=build /bin-tools /usr/local/bin/
COPY scripts/tools/*.py /usr/local/lib/hara/
ENTRYPOINT ["python3", "/usr/local/lib/hara/runtime.py"]
