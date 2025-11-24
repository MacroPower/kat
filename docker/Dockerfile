FROM alpine:latest AS build

ARG TARGETOS
ARG TARGETARCH

ENV XDG_CONFIG_HOME=/config

COPY kat /usr/local/bin/kat

RUN kat --write-config

FROM scratch

ENV XDG_CONFIG_HOME=/config

COPY --from=build /usr/local/bin/kat /kat
COPY --from=build /config /config

WORKDIR /data

ENTRYPOINT ["/kat"]
