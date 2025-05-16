FROM scratch

COPY go_template /usr/local/bin/

ENTRYPOINT ["go_template"]
