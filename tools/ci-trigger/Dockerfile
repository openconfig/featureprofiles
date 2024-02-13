FROM golang:1.21 as builder
WORKDIR /app
COPY . /app/
RUN CGO_ENABLED=0 GOOS=linux go build -mod=readonly -v -o server github.com/openconfig/featureprofiles/tools/ci-trigger
FROM golang:1.21-alpine
COPY --from=builder /app/server /server
ENTRYPOINT ["/server"]
CMD ["-alsologtostderr"]
