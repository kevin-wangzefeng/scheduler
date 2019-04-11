/*
Copyright 2019 The Kubernetes Authors.

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

package plugins

import (
	"sync"

	"github.com/kubernetes-sigs/kube-batch/pkg/controllers/job/plugins/env"
	"github.com/kubernetes-sigs/kube-batch/pkg/controllers/job/plugins/interface"
	"github.com/kubernetes-sigs/kube-batch/pkg/controllers/job/plugins/ssh"
)

func init() {
	RegisterPluginBuilder("ssh", ssh.New)
	RegisterPluginBuilder("env", env.New)
}

var pluginMutex sync.Mutex

// Plugin management
var pluginBuilders = map[string]PluginBuilder{}

// PluginBuilder is a function type that receives clientset and returns interface
type PluginBuilder func(_interface.PluginClientset, []string) _interface.PluginInterface

// RegisterPluginBuilder is used to register a pluginBuilder
func RegisterPluginBuilder(name string, pc func(_interface.PluginClientset, []string) _interface.PluginInterface) {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	pluginBuilders[name] = pc
}

// GetPluginBuilder is used to get pluginBuilder from it's name
func GetPluginBuilder(name string) (PluginBuilder, bool) {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	pb, found := pluginBuilders[name]
	return pb, found
}
