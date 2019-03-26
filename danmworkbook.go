// Copyright (c) 2019 Nokia
//
// Author: Anand Nayak
// Email: anand.nayak@nokia.com
//

package main

import (
	"encoding/json"
	"errors"
	"strings"

	danmtypes "github.com/nokia/danm/pkg/crd/apis/danm/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	danmApiPath            = "danm.k8s.io"
	danmIfDefinitionSyntax = danmApiPath + "/interfaces"
	v1Endpoint             = "/api/v1/"
	kubeConf               string
	defaultNetworkName     = "default"
)

type podMetadata struct {
	nameSpace  string
	interfaces []danmtypes.Interface
}

var danmStaticIPNetworks []string

func parsePodMetada(meta *metav1.ObjectMeta, podmeta *podMetadata) error {
	var ifaces []danmtypes.Interface
	for key, val := range meta.Annotations {
		if strings.Contains(key, danmIfDefinitionSyntax) {
			err := json.Unmarshal([]byte(val), &ifaces)
			if err != nil {
				return errors.New("Can't create network interfaces for Pod: " + meta.Name + " due to badly formatted " + danmIfDefinitionSyntax + " definition in Pod annotation")
			}
			break
		}
	}
	if len(ifaces) == 0 {
		ifaces = []danmtypes.Interface{{Network: defaultNetworkName}}
	}
	podmeta.interfaces = ifaces
	podmeta.nameSpace = meta.Namespace
	return nil
}

func danmStaticIPaddress(podmeta *podMetadata) {
	for index := len(podmeta.interfaces) - 1; index >= 0; index-- {
		if podmeta.interfaces[index].Ip != "" && podmeta.interfaces[index].Ip == "dynamic" {
			podmeta.interfaces = append(podmeta.interfaces[:index], podmeta.interfaces[index+1:]...)
		}
		if podmeta.interfaces[index].Ip6 != "" && podmeta.interfaces[index].Ip == "dynamic" {
			podmeta.interfaces = append(podmeta.interfaces[:index], podmeta.interfaces[index+1:]...)
		}
	}
}

func getDanmEp(staticip string, ipversion string) (danmtypes.DanmEpSpec, error) {
	result, err := danmclient.DanmV1().DanmEps("").List(metav1.ListOptions{})
	if err != nil {
		log.Println("cannot get list of eps:" + err.Error())
		return danmtypes.DanmEpSpec{}, err
	}
	eplist := result.Items
	for _, ep := range eplist {
		if ipversion == "IPv4" && ep.Spec.Iface.Address == staticip {
			return ep.Spec, nil
		}
		if ipversion == "IPv6" && ep.Spec.Iface.AddressIPv6 == staticip {
			return ep.Spec, nil
		}
	}
	return danmtypes.DanmEpSpec{}, nil
}
func checkForExistingStaticIPInDanmEpList(podmeta *podMetadata) (danmtypes.DanmEpSpec, error) {
	var danmep danmtypes.DanmEpSpec
	for _, danmNw := range podmeta.interfaces {
		var err error
		if danmNw.Ip != "" {
			danmep, err = getDanmEp(danmNw.Ip, "IPv4")
		}
		if danmNw.Ip6 != "" {
			danmep, err = getDanmEp(danmNw.Ip, "IPv6")
		}
		if err != nil {
			return danmep, err
		}
	}
	return danmep, nil
}

func IskubeNodesReady(hostname string) (bool, error) {

	nodeinfo, err := clientset.CoreV1().Nodes().Get(hostname, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	if nodeinfo.Status.Phase == "Running" {
		return true, nil
	}

	return false, nil
}

func danmStaticIPValidation(metadata *metav1.ObjectMeta) (bool, error) {
	podmeta := &podMetadata{}
	var danmep danmtypes.DanmEpSpec
	var err error
	if err := parsePodMetada(metadata, podmeta); err != nil {
		return true, err
	}
	danmStaticIPaddress(podmeta)
	if len(podmeta.interfaces) > 0 {
		danmep, err = checkForExistingStaticIPInDanmEpList(podmeta)
		if err != nil {
			return true, err
			//check for empty struct
		} else if danmep.EndpointID == "" {
			return false, nil
		}
	}
	// check for Host having danmep is Ready or NotReady
	status, err := IskubeNodesReady(danmep.Host)
	if err != nil {
		return true, err
	} else if status {
		return false, nil
	}
	return false, nil
}
