FROM alpine:latest
ENTRYPOINT ["/kat"]
COPY kat /
WORKDIR /data
RUN /kat --write-config
