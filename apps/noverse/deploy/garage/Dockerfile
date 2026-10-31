FROM dxflrs/garage:v2.5.0 AS garage
FROM alpine:3.22
COPY --from=garage /garage /usr/local/bin/garage
COPY init.sh /usr/local/bin/garage-init
ENTRYPOINT ["/bin/sh", "/usr/local/bin/garage-init"]
