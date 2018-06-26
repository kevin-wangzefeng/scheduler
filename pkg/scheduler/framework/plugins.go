/*
Copyright 2018 The Kubernetes Authors.

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

package framework

import "sync"

var pluginMutex sync.Mutex

// Plugin management
var pluginBuilders []func() Plugin

func RegisterPluginBuilder(pc func() Plugin) {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	pluginBuilders = append(pluginBuilders, pc)
}

func CleanupPluginBuilders() {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	pluginBuilders = []func() Plugin{}
}

// Action management
var actionMap = map[string]Action{}

func RegisterAction(act Action) {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	actionMap[act.Name()] = act
}

func GetAction(name string) (Action, bool) {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()

	act, found := actionMap[name]
	return act, found
}
