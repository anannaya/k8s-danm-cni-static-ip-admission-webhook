FROM alpine:latest

ADD k8s-danm-cni-static-ip-controller /k8s-danm-cni-static-ip-controller
ENTRYPOINT ["/k8s-danm-cni-static-ip-controller"]