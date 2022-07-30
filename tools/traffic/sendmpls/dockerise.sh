#!/bin/bash

go build -o sendmpls
docker build -t mpls_src:latest .
rm sendmpls
