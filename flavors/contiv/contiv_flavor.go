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

// Package contiv defines flavor used for Contiv-VPP agent.
package contiv

import (
	"github.com/ligato/cn-infra/core"
	"github.com/ligato/cn-infra/flavors/local"

	"github.com/contiv/contiv-vpp/plugins/cnigrpc"
	"github.com/golang/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
	local_sync "github.com/ligato/cn-infra/datasync/kvdbsync/local"
	"github.com/ligato/vpp-agent/clientv1/linux/localclient"
	"github.com/ligato/vpp-agent/flavors/vpp"
	"github.com/ligato/vpp-agent/plugins/defaultplugins"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/linuxplugin"
	"github.com/namsral/flag"
)

// FlavorContiv glues together multiple plugins to manage VPP and Linux
// configuration using the local client.
type FlavorContiv struct {
	*local.FlavorLocal
	LinuxLocalClient localclient.Plugin
	GoVPP            govppmux.GOVPPPlugin
	Linux            linuxplugin.Plugin
	VPP              defaultplugins.Plugin
	CNI              cnigrpc.Plugin
	injected         bool
}

// Inject sets inter-plugin references.
func (f *FlavorContiv) Inject() bool {
	if f.injected {
		return false
	}
	f.injected = true

	if f.FlavorLocal == nil {
		f.FlavorLocal = &local.FlavorLocal{}
	}
	f.FlavorLocal.Inject()

	f.GoVPP.Deps.PluginInfraDeps = *f.FlavorLocal.InfraDeps("govpp")
	f.Linux.Watcher = &datasync.CompositeKVProtoWatcher{Adapters: []datasync.KeyValProtoWatcher{local_sync.Get()}}
	f.VPP.Watch = &datasync.CompositeKVProtoWatcher{Adapters: []datasync.KeyValProtoWatcher{local_sync.Get()}}
	f.VPP.Deps.PluginInfraDeps = *f.FlavorLocal.InfraDeps("default-plugins")
	f.VPP.Deps.Linux = &f.Linux
	f.VPP.Deps.GoVppmux = &f.GoVPP
	f.VPP.Deps.PublishStatistics = &datasync.CompositeKVProtoWriter{Adapters: []datasync.KeyProtoValWriter{&devNullWriter{}}}
	f.VPP.Deps.IfStatePub = &datasync.CompositeKVProtoWriter{Adapters: []datasync.KeyProtoValWriter{&devNullWriter{}}}
	f.CNI.Deps.PluginInfraDeps = *f.FlavorLocal.InfraDeps("cni-grpc")

	return true
}

// Plugins combines all Plugins in the flavor to a list.
func (f *FlavorContiv) Plugins() []*core.NamedPlugin {
	f.Inject()
	return core.ListPluginsInFlavor(f)
}

func declareFlags() {
	flag.String(vpp.DefaultPluginsConfFlag, vpp.DefaultPluginsConf, vpp.DefaultPluginsConfUsage)
	flag.String(vpp.IfStatePubConfFlag, vpp.IfStatePubConf, vpp.IfStatePubConfUsage)
	flag.String(vpp.GoVPPConfFlag, vpp.GoVPPConf, vpp.GoVPPConfUsage)
}

type devNullWriter struct {
}

func (d *devNullWriter) Put(key string, data proto.Message, opts ...datasync.PutOption) error {
	return nil
}
