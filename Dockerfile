# Upstream: https://github.com/router-for-me/CLIProxyAPI
# Upgrade by bumping the tag and running `npm run deploy`.
FROM eceasy/cli-proxy-api:v8.0.15

# Config and auth files are synced to/from R2 (OBJECTSTORE_* env vars);
# /data is only a local cache and is lost when the container sleeps.
ENV WRITABLE_PATH=/data \
    OBJECTSTORE_LOCAL_PATH=/data \
    TZ=UTC

RUN mkdir -p /data

EXPOSE 8317
