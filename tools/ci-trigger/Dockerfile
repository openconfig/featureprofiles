FROM golang:1.20 as builder
WORKDIR /app
COPY . /app/
RUN CGO_ENABLED=0 GOOS=linux go build -mod=readonly -v -o server github.com/openconfig/featureprofiles/tools/ci-trigger
FROM alpine:3
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/server /server
CMD ["/server"]
