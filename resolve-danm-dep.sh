#!/bin/sh -ex

get_latest_release() {
  curl --silent "https://api.github.com/repos/$1/releases/latest" | # Get latest release from GitHub api
    grep '"tag_name":' |                                            # Get tag line
    sed -E 's/.*"([^"]+)".*/\1/'                                    # Pluck JSON value
}

danmrelease=`get_latest_release "nokia/danm"`
curl -L https://github.com/nokia/danm/archive/$danmrelease.tar.gz | tar zx
if [[ $? -ne 0 ]];
then
    echo "Failed to download danm release"
    exit 1
fi
export CGO_ENABLED=0
export GOOS=linux
mkdir -p $GOPATH/src/github.com/nokia/danm/
cp -af danm-`echo $danmrelease | tr -d "v"`/*  $GOPATH/src/github.com/nokia/danm/
go get -d github.com/vishvananda/netlink
go get github.com/containernetworking/plugins/pkg/ns
go get github.com/golang/groupcache/lru
go get k8s.io/code-generator/cmd/deepcopy-gen
go get k8s.io/code-generator/cmd/client-gen
go get k8s.io/code-generator/cmd/lister-gen
go get k8s.io/code-generator/cmd/informer-gen
export PATH=$PATH:${GOPATH}/bin
deepcopy-gen -v5 --alsologtostderr --input-dirs github.com/nokia/danm/pkg/crd/apis/danm/v1 -O zz_generated.deepcopy --bounding-dirs github.com/nokia/danm/pkg/crd/apis
client-gen -v5 --alsologtostderr --clientset-name versioned --input-base "" --input github.com/nokia/danm/pkg/crd/apis/danm/v1 --clientset-path github.com/nokia/danm/pkg/crd/client/clientset
lister-gen -v5 --alsologtostderr --input-dirs github.com/nokia/danm/pkg/crd/apis/danm/v1 --output-package github.com/nokia/danm/pkg/crd/client/listers
informer-gen -v5 --alsologtostderr --input-dirs github.com/nokia/danm/pkg/crd/apis/danm/v1 --versioned-clientset-package github.com/nokia/danm/pkg/crd/client/clientset/versioned --listers-package github.com/nokia/danm/pkg/crd/client/listers --output-package github.com/nokia/danm/pkg/crd/client/informers
