FROM golang:1.18-alpine
RUN apk add libpcap-dev libpcap
RUN apk add build-base
RUN apk add zsh

# Run docker build from root of featureprofiles.
RUN mkdir -p /go/src/github.com/openconfig/featureprofiles
WORKDIR /go/src/github.com/openconfig/featureprofiles
COPY go.mod .
COPY go.sum .
RUN mkdir tools
COPY tools/ tools/
WORKDIR tools/traffic/simpleate
RUN go build -o simpleate
RUN mkdir /app
RUN mv simpleate /app
RUN mkdir /data
COPY tools/traffic/simpleate/testdata/* /data/

EXPOSE 40051

CMD ["/app/simpleate", "-alsologtostderr", "-v=2", "-port=40051", "-telemetry_port=50051", "-certfile=/data/cert.pem", "-keyfile=/data/key.pem"]
