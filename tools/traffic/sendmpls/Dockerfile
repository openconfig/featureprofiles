FROM alpine:latest
RUN mkdir /app
WORKDIR /app
COPY sendmpls .
CMD ["/app/sendmpls", "-dst_mac=00:02:02:02:02:02", "-labels='42,43,44'", "-interface veth0"]
