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
	// genericv1alpha1 "github.com/kkohtaka/namingway/pkg/apis/generic/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PacketDeviceSpec defines the desired state of PacketDevice
type PacketDeviceSpec struct {
	ID string `json:"id"`
}

// PacketDeviceStatus defines the observed state of PacketDevice
type PacketDeviceStatus struct {
	ProjectName       string   `json:"projectName,omitempty"`
	Hostname          string   `json:"hostname,omitempty"`
	PublicIPAddresses []string `json:"publicIPAddresses,omitempty"`

	DNSRecordRef *DNSRecordRef `json:"dnsRecordRef,omitempty"`
}

// DNSRecordRef defines a reference to of DNSRecord resource
type DNSRecordRef struct {
	Name string `json:"name,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PacketDevice is the Schema for the packetdevices API
// +k8s:openapi-gen=true
type PacketDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PacketDeviceSpec   `json:"spec,omitempty"`
	Status PacketDeviceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PacketDeviceList contains a list of PacketDevice
type PacketDeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PacketDevice `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PacketDevice{}, &PacketDeviceList{})
}
