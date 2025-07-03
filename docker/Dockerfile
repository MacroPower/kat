FROM alpine:latest

ARG TARGETOS
ARG TARGETARCH

ENV XDG_CONFIG_HOME=/config

COPY kat /

RUN /kat --write-config

WORKDIR /data

ENTRYPOINT ["/kat"]
