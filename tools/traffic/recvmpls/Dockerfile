FROM golang:1.18-alpine
RUN apk add libpcap-dev libpcap
RUN apk add build-base
RUN apk add zsh

# Run docker build from root of featureprofiles.
RUN mkdir -p /go/src/github.com/openconfig/featureprofiles
WORKDIR /go/src/github.com/openconfig/featureprofiles
COPY go.mod .
COPY go.sum .
COPY tools/traffic/recvmpls .
RUN go build -o recvmpls
RUN mkdir /app
RUN mv recvmpls /app

CMD ["/app/recvmpls", "-alsologtostderr", "-v=2", "-interface eth1"]
