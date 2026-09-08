FROM alpine:3.22

RUN apk add --no-cache ca-certificates gzip mysql-client openssl tzdata

RUN addgroup -S kratos && adduser -S -G kratos -h /app kratos

WORKDIR /app

COPY bin/server ./server
COPY configs ./configs
COPY data/admin /opt/kratos-admin/static/admin
COPY data/uni-app /opt/kratos-admin/static/uni-app
COPY data/taro-app /opt/kratos-admin/static/taro-app
COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN chmod +x /usr/local/bin/docker-entrypoint.sh \
    && mkdir -p /app/data /app/logs /app/backups \
    && chown -R kratos:kratos /app /opt/kratos-admin /usr/local/bin/docker-entrypoint.sh

VOLUME ["/app/data", "/app/logs", "/app/backups"]

EXPOSE 6001 7001

USER kratos:kratos

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["./server", "-c", "./configs", "-e", "prod"]
