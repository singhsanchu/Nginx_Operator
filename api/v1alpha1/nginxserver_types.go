/*
Copyright 2024.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NginxServerSpec defines the desired state of NginxServer
type NginxServerSpec struct {
	PvcEnabled bool  `json:"pvcEnabled"`
	Size string `json:"size,omitempty"`
	Replica int32 `json:"replica,omitempty"`
}

// NginxServerStatus defines the observed state of NginxServer
type NginxServerStatus struct {
	// Add custom status fields as needed
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NginxServer is the Schema for the nginxservers API
type NginxServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NginxServerSpec   `json:"spec,omitempty"`
	Status NginxServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NginxServerList contains a list of NginxServer
type NginxServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NginxServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NginxServer{}, &NginxServerList{})
}
