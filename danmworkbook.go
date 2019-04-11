// Copyright (c) 2019 Nokia
//
// Author: Anand Nayak
// Email: anand.cs89.83@gmail.com
//

package main

import (
	"encoding/json"
	"errors"
	"strings"

	danmtypes "github.com/nokia/danm/pkg/crd/apis/danm/v1"
	"github.com/nokia/danm/pkg/ipam"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	danmApiPath            = "danm.k8s.io"
	danmIfDefinitionSyntax = danmApiPath + "/interfaces"
	v1Endpoint             = "/api/v1/"
	kubeConf               string
	defaultNetworkName     = "default"
	NodeStatus             = "Ready"
)

type podMetadata struct {
	interfaces []danmtypes.Interface
}

var danmStaticIPNetworks []string

func parsePodMetada(meta *metav1.ObjectMeta, podmeta *podMetadata) error {
	var ifaces []danmtypes.Interface
	log.Infof("meta.Annotations =%v", meta.Annotations)
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
	log.Infof("ifaces =%v", ifaces)
	podmeta.interfaces = ifaces
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

func getDanmEp(staticip string, ipversion string) (danmtypes.DanmEp, error) {
	result, err := danmclient.DanmV1().DanmEps("").List(metav1.ListOptions{})
	if err != nil {
		log.Println("cannot get list of eps:" + err.Error())
		return danmtypes.DanmEp{}, err
	}
	eplist := result.Items
	for _, ep := range eplist {
		log.Infof("IPAddress %s", ep.Spec.Iface.Address)
		log.Infof("MY Static IPAddress %s", staticip)
		if ipversion == "IPv4" && ep.Spec.Iface.Address == staticip {
			return ep, nil
		}
		if ipversion == "IPv6" && ep.Spec.Iface.AddressIPv6 == staticip {
			return ep, nil
		}
	}
	return danmtypes.DanmEp{}, nil
}
func checkForExistingStaticIPInDanmEpList(podmeta *podMetadata) (danmtypes.DanmEp, error) {
	var danmep danmtypes.DanmEp
	for _, danmNw := range podmeta.interfaces {
		var err error
		if danmNw.Ip != "" {
			log.Infof("Static ip :%s", danmNw.Ip)
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

//IskubeNodeReady check wheather the Kubernetes cluster node is ready to serve
func IskubeNodeReady(hostname string) (bool, error) {
	log.Infof("Get the nodeinfo")
	nodeinfo, err := clientset.CoreV1().Nodes().Get(hostname, metav1.GetOptions{})
	if err != nil {
		log.Infof("ERROR !!! Getting the node details %v", err)
		log.Errorf("Getting the node details %v", err)
		return false, err
	}
	conditionMap := make(map[v1.NodeConditionType]*v1.NodeCondition)
	NodeAllConditions := []v1.NodeConditionType{v1.NodeReady}
	for i := range nodeinfo.Status.Conditions {
		cond := nodeinfo.Status.Conditions[i]
		conditionMap[cond.Type] = &cond
	}
	var status []string
	for _, validCondition := range NodeAllConditions {
		if condition, ok := conditionMap[validCondition]; ok {
			if condition.Status == v1.ConditionTrue {
				status = append(status, string(condition.Type))
			} else {
				status = append(status, "Not"+string(condition.Type))
			}
		}
	}
	if len(status) == 0 {
		status = append(status, "Unknown")
	}
	if nodeinfo.Spec.Unschedulable {
		status = append(status, "SchedulingDisabled")
	}
	log.Infof("Node status %v", status)
	for _, value := range status {
		if value == "Ready" {
			log.Infof("The Node is still ready %s", hostname)
			return true, nil
		}
	}
	NodeStatus = "NotReady"
	return false, nil
}

func danmStaticIPValidation(metadata *metav1.ObjectMeta) (danmtypes.DanmEp, bool, error) {
	podmeta := &podMetadata{}
	var danmep danmtypes.DanmEp
	var err error
	if err := parsePodMetada(metadata, podmeta); err != nil {
		return danmep, true, err
	}
	danmStaticIPaddress(podmeta)
	if len(podmeta.interfaces) > 0 {
		log.Infof("Check for  checkForExistingStaticIPInDanmEpList")
		danmep, err = checkForExistingStaticIPInDanmEpList(podmeta)
		if err != nil {
			return danmep, true, err
			//check for empty struct
		} else if danmep.Spec.EndpointID == "" {
			log.Infof("IP not in danm endpoint")
			return danmep, false, nil
		}
	}
	// check for Host having danmep is Ready or NotReady
	log.Infof("Check for  IskubeNodeReady %s", danmep.Spec.Host)
	status, err := IskubeNodeReady(danmep.Spec.Host)
	if err != nil {
		return danmep, true, err
	} else if status {
		return danmep, false, nil
	}
	return danmep, false, nil
}

func deleteDanmEndPoint(ep danmtypes.DanmEp, namespace string) error {
	log.Infof("Deleting the danmep %s", ep.Spec.EndpointID)
	delOpts := metav1.DeleteOptions{}
	err := danmclient.DanmV1().DanmEps(namespace).Delete(ep.ObjectMeta.Name, &delOpts)
	if err != nil {
		return err
	}
	return nil
}

func deleteDanmStaticIP(netInfo *danmtypes.DanmNet, epspec danmtypes.DanmEpSpec) {
	log.Infof("Deleting the DanmStaticIP %s%s", epspec.Iface.Address, epspec.Iface.AddressIPv6)
	ipam.GarbageCollectIps(danmclient, netInfo, epspec.Iface.Address, epspec.Iface.AddressIPv6)
}
