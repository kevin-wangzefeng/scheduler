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
	"fmt"
	"sync"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	clientv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	arbv1 "github.com/kubernetes-incubator/kube-arbitrator/pkg/apis/v1"
	"github.com/kubernetes-incubator/kube-arbitrator/pkg/client"
	informerfactory "github.com/kubernetes-incubator/kube-arbitrator/pkg/client/informers"
	arbclient "github.com/kubernetes-incubator/kube-arbitrator/pkg/client/informers/v1"
)

// New returns a Cache implementation.
func New(config *rest.Config) Cache {
	return newSchedulerCache(config)
}

type schedulerCache struct {
	sync.Mutex

	podInformer      clientv1.PodInformer
	nodeInformer     clientv1.NodeInformer
	consumerInformer arbclient.ConsumerInformer

	pods      map[string]*PodInfo
	nodes     map[string]*NodeInfo
	consumers map[string]*ConsumerInfo
}

func newSchedulerCache(config *rest.Config) *schedulerCache {
	sc := &schedulerCache{
		nodes:     make(map[string]*NodeInfo),
		pods:      make(map[string]*PodInfo),
		consumers: make(map[string]*ConsumerInfo),
	}

	kubecli := kubernetes.NewForConfigOrDie(config)
	informerFactory := informers.NewSharedInformerFactory(kubecli, 0)

	// create informer for node information
	sc.nodeInformer = informerFactory.Core().V1().Nodes()
	sc.nodeInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    sc.AddNode,
			UpdateFunc: sc.UpdateNode,
			DeleteFunc: sc.DeleteNode,
		},
		0,
	)

	// create informer for pod information
	sc.podInformer = informerFactory.Core().V1().Pods()
	sc.podInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *v1.Pod:
					return nonTerminatedPod(t)
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    sc.AddPod,
				UpdateFunc: sc.UpdatePod,
				DeleteFunc: sc.DeletePod,
			},
		})

	// create consumer informer
	consumerClient, _, err := client.NewClient(config)
	if err != nil {
		panic(err)
	}

	consumerInformerFactory := informerfactory.NewSharedInformerFactory(consumerClient, 0)
	// create informer for consumer information
	sc.consumerInformer = consumerInformerFactory.Consumer().Consumers()
	sc.consumerInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *arbv1.Consumer:
					glog.V(4).Infof("Filter consumer name(%s) namespace(%s)\n", t.Name, t.Namespace)
					return true
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    sc.AddConsumer,
				UpdateFunc: sc.UpdateConsumer,
				DeleteFunc: sc.DeleteConsumer,
			},
		})

	return sc
}

func (sc *schedulerCache) Run(stopCh <-chan struct{}) {
	go sc.podInformer.Informer().Run(stopCh)
	go sc.nodeInformer.Informer().Run(stopCh)
	go sc.consumerInformer.Informer().Run(stopCh)
}

func (sc *schedulerCache) WaitForCacheSync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh,
		sc.podInformer.Informer().HasSynced,
		sc.nodeInformer.Informer().HasSynced,
		sc.consumerInformer.Informer().HasSynced)
}

// nonTerminatedPod selects pods that are non-terminal (pending and running).
func nonTerminatedPod(pod *v1.Pod) bool {
	if pod.Status.Phase == v1.PodSucceeded ||
		pod.Status.Phase == v1.PodFailed ||
		pod.Status.Phase == v1.PodUnknown {
		return false
	}
	return true
}

// Assumes that lock is already acquired.
func (sc *schedulerCache) addPod(pod *v1.Pod) error {
	key := podKey(pod)

	if _, ok := sc.pods[key]; ok {
		return fmt.Errorf("pod %v exist", key)
	}

	pi := NewPodInfo(pod)

	if pod.Status.Phase == v1.PodRunning {
		if sc.nodes[pod.Spec.NodeName] == nil {
			sc.nodes[pod.Spec.NodeName] = NewNodeInfo(nil)
		}
		sc.nodes[pod.Spec.NodeName].AddPod(pi)
	}

	sc.pods[key] = pi

	return nil
}

// Assumes that lock is already acquired.
func (sc *schedulerCache) updatePod(oldPod, newPod *v1.Pod) error {
	if err := sc.deletePod(oldPod); err != nil {
		return err
	}
	sc.addPod(newPod)
	return nil
}

// Assumes that lock is already acquired.
func (sc *schedulerCache) deletePod(pod *v1.Pod) error {
	key := podKey(pod)

	pi, ok := sc.pods[key]
	if !ok {
		return fmt.Errorf("pod %v doesn't exist", key)
	}
	delete(sc.pods, key)

	if len(pi.NodeName) != 0 && pi.Phase == v1.PodRunning {
		node := sc.nodes[pod.Spec.NodeName]
		if node != nil {
			node.RemovePod(pi)
		}
	}

	return nil
}

func (sc *schedulerCache) AddPod(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		glog.Errorf("Cannot convert to *v1.Pod: %v", obj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	glog.V(4).Infof("Add pod(%s) into cache, status (%s)", pod.Name, pod.Status.Phase)
	err := sc.addPod(pod)
	if err != nil {
		glog.Errorf("Failed to add pod %s into cache: %v", pod.Name, err)
		return
	}
	return
}

func (sc *schedulerCache) UpdatePod(oldObj, newObj interface{}) {
	oldPod, ok := oldObj.(*v1.Pod)
	if !ok {
		glog.Errorf("Cannot convert oldObj to *v1.Pod: %v", oldObj)
		return
	}
	newPod, ok := newObj.(*v1.Pod)
	if !ok {
		glog.Errorf("Cannot convert newObj to *v1.Pod: %v", newObj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	glog.V(4).Infof("Update oldPod(%s) status(%s) newPod(%s) status(%s) in cache", oldPod.Name, oldPod.Status.Phase, newPod.Name, newPod.Status.Phase)
	err := sc.updatePod(oldPod, newPod)
	if err != nil {
		glog.Errorf("Failed to update pod %v in cache: %v", oldPod.Name, err)
		return
	}
	return
}

func (sc *schedulerCache) DeletePod(obj interface{}) {
	var pod *v1.Pod
	switch t := obj.(type) {
	case *v1.Pod:
		pod = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		pod, ok = t.Obj.(*v1.Pod)
		if !ok {
			glog.Errorf("Cannot convert to *v1.Pod: %v", t.Obj)
			return
		}
	default:
		glog.Errorf("Cannot convert to *v1.Pod: %v", t)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	glog.V(4).Infof("Delete pod(%s) status(%s) from cache", pod.Name, pod.Status.Phase)
	err := sc.deletePod(pod)
	if err != nil {
		glog.Errorf("Failed to delete pod %v from cache: %v", pod.Name, err)
		return
	}
	return
}

// Assumes that lock is already acquired.
func (sc *schedulerCache) addNode(node *v1.Node) error {
	if sc.nodes[node.Name] != nil {
		sc.nodes[node.Name].SetNode(node)
	} else {
		sc.nodes[node.Name] = NewNodeInfo(node)
	}

	return nil
}

// Assumes that lock is already acquired.
func (sc *schedulerCache) updateNode(oldNode, newNode *v1.Node) error {
	// Did not delete the old node, just update related info, e.g. allocatable.
	if sc.nodes[newNode.Name] != nil {
		sc.nodes[newNode.Name].SetNode(newNode)
		return nil
	}

	return fmt.Errorf("node <%s> does not exist", newNode.Name)
}

// Assumes that lock is already acquired.
func (sc *schedulerCache) deleteNode(node *v1.Node) error {
	if _, ok := sc.nodes[node.Name]; !ok {
		return fmt.Errorf("node <%s> does not exist", node.Name)
	}
	delete(sc.nodes, node.Name)
	return nil
}

func (sc *schedulerCache) AddNode(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if !ok {
		glog.Errorf("Cannot convert to *v1.Node: %v", obj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	glog.V(4).Infof("Add node(%s) into cache", node.Name)
	err := sc.addNode(node)
	if err != nil {
		glog.Errorf("Failed to add node %s into cache: %v", node.Name, err)
		return
	}
	return
}

func (sc *schedulerCache) UpdateNode(oldObj, newObj interface{}) {
	oldNode, ok := oldObj.(*v1.Node)
	if !ok {
		glog.Errorf("Cannot convert oldObj to *v1.Node: %v", oldObj)
		return
	}
	newNode, ok := newObj.(*v1.Node)
	if !ok {
		glog.Errorf("Cannot convert newObj to *v1.Node: %v", newObj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	glog.V(4).Infof("Update oldNode(%s) newNode(%s) in cache", oldNode.Name, newNode.Name)
	err := sc.updateNode(oldNode, newNode)
	if err != nil {
		glog.Errorf("Failed to update node %v in cache: %v", oldNode.Name, err)
		return
	}
	return
}

func (sc *schedulerCache) DeleteNode(obj interface{}) {
	var node *v1.Node
	switch t := obj.(type) {
	case *v1.Node:
		node = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		node, ok = t.Obj.(*v1.Node)
		if !ok {
			glog.Errorf("Cannot convert to *v1.Node: %v", t.Obj)
			return
		}
	default:
		glog.Errorf("Cannot convert to *v1.Node: %v", t)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	glog.V(4).Infof("Delete node(%s) from cache", node.Name)
	err := sc.deleteNode(node)
	if err != nil {
		glog.Errorf("Failed to delete node %s from cache: %v", node.Name, err)
		return
	}
	return
}

// Assumes that lock is already acquired.
func (sc *schedulerCache) addConsumer(consumer *arbv1.Consumer) error {
	if _, ok := sc.consumers[consumer.Name]; ok {
		return fmt.Errorf("consumer %v exist", consumer.Name)
	}

	sc.consumers[consumer.Name] = NewConsumerInfo(consumer)
	return nil
}

// Assumes that lock is already acquired.
func (sc *schedulerCache) updateConsumer(oldConsumer, newConsumer *arbv1.Consumer) error {
	if err := sc.deleteConsumer(oldConsumer); err != nil {
		return err
	}
	sc.addConsumer(newConsumer)
	return nil
}

// Assumes that lock is already acquired.
func (sc *schedulerCache) deleteConsumer(consumer *arbv1.Consumer) error {
	if _, ok := sc.consumers[consumer.Name]; !ok {
		return fmt.Errorf("consumer %v doesn't exist", consumer.Name)
	}
	delete(sc.consumers, consumer.Name)
	return nil
}

func (sc *schedulerCache) AddConsumer(obj interface{}) {
	consumer, ok := obj.(*arbv1.Consumer)
	if !ok {
		glog.Errorf("Cannot convert to *arbv1.Consumer: %v", obj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	glog.V(4).Infof("Add consumer(%s) into cache, spec(%#v)", consumer.Name, consumer.Spec)
	err := sc.addConsumer(consumer)
	if err != nil {
		glog.Errorf("Failed to add consumer %s into cache: %v", consumer.Name, err)
		return
	}
	return
}

func (sc *schedulerCache) UpdateConsumer(oldObj, newObj interface{}) {
	oldConsumer, ok := oldObj.(*arbv1.Consumer)
	if !ok {
		glog.Errorf("Cannot convert oldObj to *arbv1.Consumer: %v", oldObj)
		return
	}
	newConsumer, ok := newObj.(*arbv1.Consumer)
	if !ok {
		glog.Errorf("Cannot convert newObj to *arbv1.Consumer: %v", newObj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	glog.V(4).Infof("Update oldConsumer(%s) in cache, spec(%#v)", oldConsumer.Name, oldConsumer.Spec)
	glog.V(4).Infof("Update newConsumer(%s) in cache, spec(%#v)", newConsumer.Name, newConsumer.Spec)
	err := sc.updateConsumer(oldConsumer, newConsumer)
	if err != nil {
		glog.Errorf("Failed to update consumer %s into cache: %v", oldConsumer.Name, err)
		return
	}
	return
}

func (sc *schedulerCache) DeleteConsumer(obj interface{}) {
	var consumer *arbv1.Consumer
	switch t := obj.(type) {
	case *arbv1.Consumer:
		consumer = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		consumer, ok = t.Obj.(*arbv1.Consumer)
		if !ok {
			glog.Errorf("Cannot convert to *v1.Consumer: %v", t.Obj)
			return
		}
	default:
		glog.Errorf("Cannot convert to *v1.Consumer: %v", t)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.deleteConsumer(consumer)
	if err != nil {
		glog.Errorf("Failed to delete consumer %s from cache: %v", consumer.Name, err)
		return
	}
	return
}

func (sc *schedulerCache) PodInformer() clientv1.PodInformer {
	return sc.podInformer
}

func (sc *schedulerCache) NodeInformer() clientv1.NodeInformer {
	return sc.nodeInformer
}

func (sc *schedulerCache) QueueInformer() arbclient.ConsumerInformer {
	return sc.consumerInformer
}

func (sc *schedulerCache) Snapshot() *CacheSnapshot {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	snapshot := &CacheSnapshot{
		Nodes:     make([]*NodeInfo, 0, len(sc.nodes)),
		Pods:      make([]*PodInfo, 0, len(sc.pods)),
		Consumers: make([]*ConsumerInfo, 0, len(sc.consumers)),
	}

	for _, value := range sc.nodes {
		snapshot.Nodes = append(snapshot.Nodes, value.Clone())
	}
	for _, value := range sc.pods {
		snapshot.Pods = append(snapshot.Pods, value.Clone())
	}
	for _, value := range sc.consumers {
		snapshot.Consumers = append(snapshot.Consumers, value.Clone())
	}
	return snapshot
}

func (sc schedulerCache) String() string {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	str := "Cache:\n"

	if len(sc.nodes) != 0 {
		str = str + "Nodes:\n"
		for _, n := range sc.nodes {
			str = str + fmt.Sprintf("\t %s: idle(%v) used(%v) allocatable(%v) pods(%d)\n",
				n.Name, n.Idle, n.Used, n.Allocatable, len(n.Pods))
		}
	}

	if len(sc.pods) != 0 {
		str = str + "Pods:\n"
		for _, p := range sc.pods {
			str = str + fmt.Sprintf("\t %s/%s: phase (%s), node (%s), request (%v)\n",
				p.Namespace, p.Name, p.Phase, p.NodeName, p.Request)
		}
	}

	return str

}
