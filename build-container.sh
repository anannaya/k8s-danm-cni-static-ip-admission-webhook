#!/bin/bash

GOOS=linux go build -a --ldflags '-extldflags "-static"' -tags netgo -installsuffix netgo -o k8s-danm-cni-static-ip-controller .

docker build -t k8s-danm-cni-static-ip-controller:0.0.1 .