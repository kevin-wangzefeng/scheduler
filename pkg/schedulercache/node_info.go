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

package schedulercache

import (
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	clientset "k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/listers/core/v1"
	clientcache "k8s.io/client-go/tools/cache"
)

// NodeInfo is node level aggregated information.
type NodeInfo struct {
	// Overall node information.
	name string
	node *v1.Node
}

func (n *NodeInfo) Name() string {
	return n.name
}

// Returns overall information about this node.
func (n *NodeInfo) Node() *v1.Node {
	return n.node
}

func (n *NodeInfo) Clone() *NodeInfo {
	clone := &NodeInfo{
		name: n.name,
		node: n.node.DeepCopy(),
	}
	return clone
}

// getPodKey returns the string key of a pod.
func getPodKey(pod *v1.Pod) (string, error) {
	return clientcache.MetaNamespaceKeyFunc(pod)
}

func NodeLister(client clientset.Interface, stopChannel <-chan struct{}) ([]*v1.Node, error) {
	nl := GetNodeLister(client, stopChannel)
	nodes, err := nl.List(labels.Everything())
	if err != nil {
		return []*v1.Node{}, err
	}
	return nodes, err
}

func GetNodeLister(client clientset.Interface, stopChannel <-chan struct{}) corev1.NodeLister {
	listWatcher := clientcache.NewListWatchFromClient(client.Core().RESTClient(), "nodes", v1.NamespaceAll, fields.Everything())
	store := clientcache.NewIndexer(clientcache.MetaNamespaceKeyFunc, clientcache.Indexers{clientcache.NamespaceIndex: clientcache.MetaNamespaceIndexFunc})
	nodeLister := corev1.NewNodeLister(store)
	reflector := clientcache.NewReflector(listWatcher, &v1.Node{}, store, time.Hour)
	reflector.Run(stopChannel)

	return nodeLister
}
