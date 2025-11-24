FROM alpine:latest AS build

ARG TARGETOS
ARG TARGETARCH

ENV XDG_CONFIG_HOME=/config

RUN apk add --no-cache binutils

COPY kat /usr/local/bin/kat

RUN echo "=== Binary Info ===" \
  && ls -lh /usr/local/bin/kat \
  && echo "=== Architecture Check ===" \
  && readelf -h /usr/local/bin/kat | grep -E "(Class|Machine)" \
  && echo "=== Dynamic Dependencies ===" \
  && (readelf -d /usr/local/bin/kat 2>&1 || echo "No dynamic section (static binary)") \
  && echo "=== Go Version ===" \
  && (strings /usr/local/bin/kat | grep -E "^go[0-9]+\.[0-9]+" | head -1) \
  && echo "=== Container Arch ===" \
  && uname -m \
  && echo "==================" \
  && /usr/local/bin/kat --write-config

FROM scratch

ENV XDG_CONFIG_HOME=/config

COPY --from=build /usr/local/bin/kat /kat
COPY --from=build /config /config

WORKDIR /data

ENTRYPOINT ["/kat"]
