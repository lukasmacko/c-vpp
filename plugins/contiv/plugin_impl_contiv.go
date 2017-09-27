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

//go:generate protoc -I ./model/cni --go_out=plugins=grpc:./model/cni ./model/cni/cni.proto

package contiv

import (
	"github.com/contiv/contiv-vpp/plugins/contiv/model/cni"
	"github.com/ligato/cn-infra/flavors/local"
	"github.com/ligato/cn-infra/rpc/grpc"
)

// Plugin
type Plugin struct {
	Deps

	cniServer *remoteCNIserver
}

type Deps struct {
	local.PluginInfraDeps
	GRPC grpc.Server
}

func (plugin *Plugin) Init() error {
	plugin.cniServer = newRemoteCNIServer(plugin.Log)
	cni.RegisterRemoteCNIServer(plugin.GRPC.Server(), plugin.cniServer)
	return nil
}

func (plugin *Plugin) Close() error {
	return nil
}
