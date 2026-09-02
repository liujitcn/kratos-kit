FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY bin/server ./server
COPY configs ./configs
COPY root.key ./root.key
COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN chmod +x /usr/local/bin/docker-entrypoint.sh \
    && mkdir -p /app/data

VOLUME ["/app/data"]

EXPOSE 7001 6001
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["./server", "-c", "./configs", "-e", "dev"]
