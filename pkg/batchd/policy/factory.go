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

package policy

import (
	"fmt"
	"sync"

	"github.com/kubernetes-incubator/kube-arbitrator/pkg/batchd/policy/plugins/drf"
)

var policyMap = make(map[string]Interface)
var mutex sync.Mutex

func init() {
	RegisterPolicy(drf.PolicyName, drf.New())
}

func New(name string) (Interface, error) {
	mutex.Lock()
	defer mutex.Unlock()

	policy, found := policyMap[name]
	if !found {
		return nil, fmt.Errorf("invalid policy name %s\n", name)
	}

	return policy, nil
}

func RegisterPolicy(name string, policy Interface) error {
	mutex.Lock()
	defer mutex.Unlock()

	policyMap[name] = policy

	return nil
}
