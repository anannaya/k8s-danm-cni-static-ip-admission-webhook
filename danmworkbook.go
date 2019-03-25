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
	interfaces []Interface
}

var danmStaticIPNetworks []string

func parsePodMetada(meta *metav1.ObjectMeta, podmeta *podMetadata) error {
	var ifaces []Interface
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
		ifaces = []Interface{{Network: defaultNetworkName}}
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

func getDanmEp(staticip string) {

}
func checkForExistingStaticIPInDanmEpList(podmeta *podMetadata) {
	for _, danmNw := range podmeta.interfaces {
		if danmNw.Ip != "" {
			getDanmEp(danmNw.Ip)
		}
		if danmNw.Ip6 != "" {
			getDanmEp(danmNw.Ip)
		}
	}

}

func checkFordanmStaticIPOnPodWorkloads(metadata *metav1.ObjectMeta) (bool, error) {
	podmeta := &podMetadata{}
	if err := parsePodMetada(metadata, podmeta); err != nil {
		return true, err
	}
	danmStaticIPaddress(podmeta)
	if len(podmeta.interfaces) > 0 {
		checkForExistingStaticIPInDanmEpList(podmeta)
	}
	return false, nil
}
