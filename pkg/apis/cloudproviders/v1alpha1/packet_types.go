/*
Copyright 2018 Kazumasa Kohtaka <kkohtaka@gmail.com>.

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

// PacketSpec defines the desired state of Packet
type PacketSpec struct {
	ProjectID string    `json:"projectID"`
	Secret    SecretRef `json:"secretRef"`
}

// SecretRef defines a reference of Secret resource
type SecretRef struct {
	SecretName string `json:"secretName"`
}

// PacketStatus defines the observed state of Packet
type PacketStatus struct {
	ProjectName string `json:"projectName"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Packet is the Schema for the packets API
// +k8s:openapi-gen=true
type Packet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PacketSpec   `json:"spec,omitempty"`
	Status PacketStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PacketList contains a list of Packet
type PacketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Packet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Packet{}, &PacketList{})
}
