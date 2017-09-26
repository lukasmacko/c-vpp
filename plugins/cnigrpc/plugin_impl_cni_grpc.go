// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cnigrpc

import "github.com/ligato/cn-infra/flavors/local"
import "github.com/ligato/vpp-agent/plugins/defaultplugins/ifplugin/model/interfaces"
import "github.com/ligato/vpp-agent/clientv1/defaultplugins/localclient"

// Plugin
type Plugin struct {
	Deps
}

type Deps struct {
	local.PluginInfraDeps
}

func (plugin *Plugin) Init() error {
	return nil
}

func (plugin *Plugin) AfterInit() error {
	return localclient.DataChangeRequest(plugin.PluginName).
		Put().
		Interface(&memif1AsMaster).
		Send().ReceiveReply()

}

func (plugin *Plugin) Close() error {
	return nil
}

// memif1AsMaster is an example of a memory interface configuration. (Master=true, with IPv4 address).
var memif1AsMaster = interfaces.Interfaces_Interface{
	Name:    "memif1",
	Type:    interfaces.InterfaceType_MEMORY_INTERFACE,
	Enabled: true,
	Memif: &interfaces.Interfaces_Interface_Memif{
		Id:             1,
		Master:         true,
		SocketFilename: "/tmp/memif1.sock",
	},
	Mtu:         1500,
	IpAddresses: []string{"192.168.1.1/24"},
}
