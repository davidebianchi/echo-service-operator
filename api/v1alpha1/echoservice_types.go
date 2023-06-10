/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EchoServiceConfig struct {
	ResponseDelay string `json:"responseDelay,omitempty"`
}

// EchoServiceSpec defines the desired state of EchoService
type EchoServiceSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of EchoService. Edit echoservice_types.go to remove/update
	Image    string            `json:"image,omitempty"`
	Version  string            `json:"version,omitempty"`
	Replicas *int32            `json:"replicas,omitempty"`
	Config   EchoServiceConfig `json:"config,omitempty"`
}

// EchoServiceStatus defines the observed state of EchoService
type EchoServiceStatus struct {
	// Important: Run "make" to regenerate code after modifying this file

	// TODO:
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// EchoService is the Schema for the echoservices API
type EchoService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EchoServiceSpec   `json:"spec,omitempty"`
	Status EchoServiceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EchoServiceList contains a list of EchoService
type EchoServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EchoService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EchoService{}, &EchoServiceList{})
}

func (e EchoService) GetImage() string {
	image := "davidebianchi/echo-service"
	version := "latest"

	if e.Spec.Image != "" {
		image = e.Spec.Image
	}
	if !strings.Contains(image, ":") && e.Spec.Version != "" {
		version = e.Spec.Version
	}
	return fmt.Sprintf("%s:%s", image, version)
}

func (e EchoService) GetReplicas() *int32 {
	return e.Spec.Replicas
}