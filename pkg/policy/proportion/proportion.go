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

package proportion

import (
	"github.com/golang/glog"
	apiv1 "github.com/kubernetes-incubator/kube-arbitrator/pkg/apis/v1"
	"github.com/kubernetes-incubator/kube-arbitrator/pkg/policy"
	"github.com/kubernetes-incubator/kube-arbitrator/pkg/schedulercache"

	"k8s.io/apimachinery/pkg/api/resource"
)

type proportionScheduler struct {
	name string
}

func New() policy.Interface {
	return newProportionScheduler("proportion-scheduler")
}

func newProportionScheduler(name string) *proportionScheduler {
	return &proportionScheduler{
		name: name,
	}
}

func (ps *proportionScheduler) Name() string {
	return ps.name
}

func (ps *proportionScheduler) Initialize() {
	// TODO
}

func (ps *proportionScheduler) Group(
	jobs []*schedulercache.ResourceQuotaAllocatorInfo,
) map[string][]*schedulercache.ResourceQuotaAllocatorInfo {
	groups := make(map[string][]*schedulercache.ResourceQuotaAllocatorInfo)
	for _, job := range jobs {
		groups[job.Allocator().Namespace] = append(groups[job.Allocator().Namespace], job)
	}

	return groups
}

func (ps *proportionScheduler) Allocate(
	jobGroup map[string][]*schedulercache.ResourceQuotaAllocatorInfo,
	nodes []*schedulercache.NodeInfo,
) map[string]*schedulercache.ResourceQuotaAllocatorInfo {
	totalCPU := int64(0)
	totalMEM := int64(0)
	for _, node := range nodes {
		if cpu, ok := node.Node().Status.Capacity["cpu"]; ok {
			if capacity, ok := cpu.AsInt64(); ok {
				totalCPU += capacity
			}
		}
		if memory, ok := node.Node().Status.Capacity["memory"]; ok {
			if capacity, ok := memory.AsInt64(); ok {
				totalMEM += capacity
			}
		}
	}
	totalWeight := int64(0)
	for _, jobs := range jobGroup {
		for _, job := range jobs {
			if weight, ok := job.Allocator().Spec.Share["weight"]; ok {
				totalWeight += int64(weight.IntValue())
			}
		}
	}
	glog.V(4).Infof("proportion scheduler, total cpu %d, total memory %d, total weight %d", totalCPU, totalMEM, totalWeight)

	if totalCPU == 0 || totalMEM == 0 || totalWeight == 0 {
		glog.V(4).Info("there is no resources or allocators in cluster")
		return nil
	}

	allocatedResult := make(map[string]*schedulercache.ResourceQuotaAllocatorInfo)
	for _, jobs := range jobGroup {
		for _, job := range jobs {
			if weight, ok := job.Allocator().Spec.Share["weight"]; ok {
				allocatedResult[job.Name()] = job.Clone()
				allocatedResult[job.Name()].Allocator().Status.Share = apiv1.ResourceList{
					Resources: map[apiv1.ResourceName]resource.Quantity{
						"cpu":    *resource.NewQuantity(int64(weight.IntValue())*totalCPU/totalWeight, resource.DecimalSI),
						"memory": *resource.NewQuantity(int64(weight.IntValue())*totalMEM/totalWeight, resource.BinarySI),
					},
				}
			}
		}
	}
	return allocatedResult
}

func (ps *proportionScheduler) Assign(
	jobs []*schedulercache.ResourceQuotaAllocatorInfo,
	alloc *schedulercache.ResourceQuotaAllocatorInfo,
) *schedulercache.Resource {
	// TODO
	return nil
}

func (ps *proportionScheduler) Polish(
	job *schedulercache.ResourceQuotaAllocatorInfo,
	res *schedulercache.Resource,
) []*schedulercache.ResourceQuotaAllocatorInfo {
	// TODO
	return nil
}

func (ps *proportionScheduler) UnInitialize() {
	// TODO
}
