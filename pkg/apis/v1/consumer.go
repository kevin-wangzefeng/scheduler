/*
Copyright 2017 The Kubernetes Authors.

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

package v1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ConsumerPlural = "consumers"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Consumer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              ConsumerSpec `json:"spec"`
	//Status ConsumerStatus `json:"status,omitempty"`
}

type ConsumerSpec struct {
	Weight   int             `json:"weight"`
	Reserved v1.ResourceList `json:"reserved"`
}

type ConsumerStatus struct {
	Deserved   v1.ResourceList `json:"deserved"`
	Allocated  v1.ResourceList `json:"allocated"`
	Used       v1.ResourceList `json:"used"`
	Preempting v1.ResourceList `json:"preempting"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ConsumerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Consumer `json:"items"`
}

type ResourceName string
