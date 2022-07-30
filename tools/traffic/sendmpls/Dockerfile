FROM golang:1.18-alpine
RUN apk add libpcap-dev libpcap
RUN apk add build-base
RUN apk add zsh

# Run docker build from root of featureprofiles.
RUN mkdir -p /go/src/github.com/openconfig/featureprofiles
WORKDIR /go/src/github.com/openconfig/featureprofiles
COPY go.mod .
COPY go.sum .
COPY tools/traffic/sendmpls .
RUN go build -o sendmpls
RUN mkdir /app
RUN mv sendmpls /app

CMD ["/app/sendmpls", "-alsologtostderr","-v=2", "-interface eth1", "-dst_mac=00:02:02:02:02:02", "-labels=42,43,44"]
