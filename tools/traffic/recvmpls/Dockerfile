FROM alpine:latest
RUN mkdir /app
WORKDIR /app
COPY recvmpls .
CMD ["/app/recvmpls", "-interface eth1"]
