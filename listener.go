// Copyright (c) 2019 Nokia
//
// Author: Anand Nayak
// Email: anand.nayak@nokia.com
//

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()

	// (https://github.com/kubernetes/kubernetes/issues/57982)
	defaulter = runtime.ObjectDefaulter(runtimeScheme)
)

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1beta1.AddToScheme(runtimeScheme)
	// defaulting with webhooks:
	// https://github.com/kubernetes/kubernetes/issues/57982
	_ = v1.AddToScheme(runtimeScheme)
}

var ignoredNamespaces = []string{
	metav1.NamespaceSystem,
	metav1.NamespacePublic,
}

func mutationRequired(ignoredList []string, metadata *metav1.ObjectMeta, kind string) (bool, error) {
	// skip special kubernete system namespaces
	for _, namespace := range ignoredList {
		if metadata.Namespace == namespace {
			log.Infof("Skip mutation for %v for it' in special namespace:%v", metadata.Name, metadata.Namespace)
			return false, nil
		}
	}

	//check if the kind obect is other than replicaset/deployment
	if kind != "ReplicaSet" || kind != "Deployment" {
		return false, nil
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		// determine whether to perform mutation based on annotation for the target resource
		if _, ok := annotations[danmIfDefinitionSyntax]; !ok {
			return false, nil
		}
	}
	// clear the danm endpoint incase of static ip address and pod running node is NotReady
	return danmStaticIPValidation(metadata)

}

//main mutation process to mutate the danm crd incase of danm annotation
func mutate(admReview *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := admReview.Request
	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		log.Errorf("Could not unmarshal the raw obejct:%v", err)
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.Infof("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind.Kind, req.Namespace, req.Name, pod.Name, req.UID, req.Operation, req.UserInfo)

	// determine wheather to perform mutation
	if status, err := mutationRequired(ignoredNamespaces, &pod.ObjectMeta, req.Kind.Kind); err != nil {
		log.Errorf("Failed to check the mutation required condition:%v", err)
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else if !status {
		log.Infof("Skipping mutation for %s/%s due to policy  check", pod.Namespace, pod.Name)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	// clear the danm endpoint incase of static ip address and pod running node is NotReady
	return &v1beta1.AdmissionResponse{}
}

// webhookHandler handles the danm cni static ip validator admission webhook
func webhookHandler(rw http.ResponseWriter, req *http.Request) {
	log.Infof("Serving %s %s request for client: %s", req.Method, req.URL.Path, req.RemoteAddr)

	if req.Method != http.MethodPost {
		http.Error(rw, fmt.Sprintf("Incoming request method %s is not supported, only POST is supported", req.Method), http.StatusMethodNotAllowed)
		return
	}

	if req.URL.Path != "/" {
		http.Error(rw, fmt.Sprintf("%s 404 Not Found", req.URL.Path), http.StatusNotFound)
		return
	}

	var body []byte
	if req.Body != nil {
		if data, err := ioutil.ReadAll(req.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		log.Error("empty body")
		http.Error(rw, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := req.Header.Get("Content-Type")
	if contentType != "application/json" {
		log.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(rw, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1beta1.AdmissionResponse
	admReview := v1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &admReview); err != nil {
		log.Errorf("Can't decode the body:%v", err)
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = mutate(&admReview)
	}

	admissionReview := v1beta1.AdmissionReview{}
	if *admitAll == true {
		log.Warnf("admitAll flag is set to true. Allowing Namespace admission review request to pass without validation.")
		admissionResponse = &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Reason: metav1.StatusReason("Admitall Enabled"),
			},
		}
	}

	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if admReview.Request != nil {
			admissionReview.Response.UID = admReview.Response.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		log.Errorf("Can't encode the response:%v", err)
		http.Error(rw, fmt.Sprintf("Could not encode the response : %v", err), http.StatusInternalServerError)

	}

	log.Infof("Ready to write a response...")
	if _, err := rw.Write(resp); err != nil {
		log.Errorf("Can't write a response:%v", err)
		http.Error(rw, fmt.Sprintf("Could not write a response : %v", err), http.StatusInternalServerError)
	}
}
