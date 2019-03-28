# k8s-danm-cni-static-ip-controller

k8s-danm-cni-static-ip-controller provides an admission control policy that takes care of
static ip management in case unavailability of worker node.

![flow chart](/flow.jpg)
## Implementation

This is implemented as an [External Admission Webhook](https://kubernetes.io/docs/admin/extensible-admission-controllers/#external-admission-webhooks) with the k8s-danm-cni-static-ip-controller service running as a deployment on each cluster.  

The webhook is configured to send admission review requests for *CREATE* operations on `workload` like `Pod,ReplicaSets` resources to the k8s-danm-cni-static-ip-controller service. The k8s-danm-cni-static-ip-controller service listens on a HTTPS port and on receiving such requests, it reads the annotation block from the workload(Pod) manifest,if the danm annotation available then it checks for static ip in the annotation , calls the danm apis check the pod status ,if the pod is not running and node is healthy No operation performed, if the node is not Ready then it  will cleanup the danm endpoints and static ip entry in the danm ipam.

The following resources are currently checked for existence:
- replicasets
- deployments

The k8s-danm-cni-static-ip-controller policy implementation enforces before the above listed resources creation having the static ip.
## Basic Dev Setup
1. Git clone to your local directory.
2. Build binary:
    $ ./resolve-danm-dep.sh
    $ GOOS=linux go build -a --ldflags '-extldflags "-static"' -tags netgo -installsuffix netgo -o      k8s-danm-cni-static-ip-admission-webhook .
## Command Line Args

```
USAGE:
  --admitAll     bool    True to admit all resources without validation. (default false)
  --certFile     string  The cert file for the https server. (default "/var/lib/kubernetes/kubernetes.pem")
  --clientAuth   bool    True to verify client cert/auth during TLS handshake. (default false)
  --clientCAFile string  The cluster root CA that signs the apiserver cert (default "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
  --keyFile      string  The key file for the https server. (default "/var/lib/kubernetes/kubernetes-key.pem")
  --logFile      string  Log file name and full path.
  (default "/var/log/danmlifecycle.log")
  --logLevel     string  The log level. (default "info")
  --port         string  Server port. (default "443")
```
