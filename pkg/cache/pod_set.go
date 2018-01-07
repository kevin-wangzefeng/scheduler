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

package cache

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type PodInfo struct {
	UID       types.UID
	Owner     types.UID
	Name      string
	Namespace string

	NodeName string
	Phase    v1.PodPhase

	Pod *v1.Pod

	Request *Resource
}

func NewPodInfo(pod *v1.Pod) *PodInfo {
	req := EmptyResource()

	for _, c := range pod.Spec.Containers {
		req.Add(NewResource(c.Resources.Requests))
	}

	pi := &PodInfo{
		UID:       pod.UID,
		Name:      pod.Name,
		Namespace: pod.Namespace,
		NodeName:  pod.Spec.NodeName,
		Phase:     pod.Status.Phase,

		Pod:     pod,
		Request: req,
	}

	for _, or := range pod.OwnerReferences {
		pi.Owner = or.UID
		break
	}

	return pi
}

func (pi *PodInfo) Clone() *PodInfo {
	return &PodInfo{
		UID:       pi.UID,
		Owner:     pi.Owner,
		Name:      pi.Name,
		Namespace: pi.Namespace,
		NodeName:  pi.NodeName,
		Phase:     pi.Phase,
		Pod:       pi.Pod,
		Request:   pi.Request.Clone(),
	}
}

type PodSet struct {
	metav1.ObjectMeta

	Allocated    *Resource
	TotalRequest *Resource

	Running []*PodInfo
	Pending []*PodInfo
	Others  []*PodInfo
}

func NewPodSet(uid types.UID) *PodSet {
	return &PodSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(uid),
			UID:  uid,
		},
		Allocated:    EmptyResource(),
		TotalRequest: EmptyResource(),
		Running:      make([]*PodInfo, 0),
		Pending:      make([]*PodInfo, 0),
		Others:       make([]*PodInfo, 0),
	}
}

func (ps *PodSet) AddPodInfo(pi *PodInfo) {
	switch pi.Phase {
	case v1.PodRunning:
		ps.Running = append(ps.Running, pi)
		ps.Allocated.Add(pi.Request)
		ps.TotalRequest.Add(pi.Request)
	case v1.PodPending:
		ps.Pending = append(ps.Pending, pi)
		ps.TotalRequest.Add(pi.Request)
	default:
		ps.Others = append(ps.Others, pi)
	}
}
