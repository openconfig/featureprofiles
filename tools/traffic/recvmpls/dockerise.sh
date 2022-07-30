#!/bin/bash

go build -o recvmpls
docker build -t mpls_dst:latest .
rm recvmpls
