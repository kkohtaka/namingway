/*
Copyright 2019 Kazumasa Kohtaka <kkohtaka@gmail.com>.

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

// DeviceSpec defines the desired state of Device
type DeviceSpec struct {
	ProjectID    string `json:"projectID"`
	Facility     string `json:"facility"`
	Plan         string `json:"plan"`
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	BillingCycle string `json:"billing_cicle,omitempty"`
	UserData     string `json:"userData,omitempty"`
}

// DeviceStatus defines the observed state of Device
type DeviceStatus struct {
	Ready bool `json:"ready"`

	ID          string      `json:"id"`
	State       State       `json:"state"`
	IPAddresses []IPAddress `json:"ipAddresses,omitempty"`
}

type State string

const (
	StateActive       State = "active"
	StateInactive     State = "inactive"
	StateQueued       State = "queued"
	StateProvisioning State = "provisioning"
	StateUnknown      State = ""
)

func StringToState(state string) State {
	switch state {
	case string(StateActive):
		return StateActive
	case string(StateInactive):
		return StateInactive
	case string(StateQueued):
		return StateQueued
	case string(StateProvisioning):
		return StateProvisioning
	default:
		return StateUnknown
	}
}

type IPAddress struct {
	ID            string `json:"id"`
	Address       string `json:"address"`
	Gateway       string `json:"gateway"`
	Network       string `json:"network"`
	AddressFamily int    `json:"addressFamily"`
	Netmask       string `json:"netmask"`
	Public        bool   `json:"public"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Device is the Schema for the devices API
// +k8s:openapi-gen=true
type Device struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeviceSpec   `json:"spec,omitempty"`
	Status DeviceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DeviceList contains a list of Device
type DeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Device `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Device{}, &DeviceList{})
}
