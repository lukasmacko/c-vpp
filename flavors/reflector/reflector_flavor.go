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

// Package reflector defines flavor used for PolicyReflector agent.
package reflector

import (
	"github.com/contiv/contiv-vpp/plugins/policyreflector"
	"github.com/ligato/cn-infra/core"
	"github.com/ligato/cn-infra/flavors/connectors"
	"github.com/ligato/cn-infra/flavors/local"
)

// ReflectorFlavor glues together multiple plugins to watch selected k8s
// resources and causes all changes to be reflected in a given store.
type FlavorReflector struct {
	// Local flavor is used to access the Infra (logger, service label, status check)
	*local.FlavorLocal
	// Connectors to various data stores.
	*connectors.AllConnectorsFlavor
	// K8s policy reflector.
	PolicyReflector policyreflector.Plugin

	injected bool
}

// Inject sets inter-plugin references.
func (f *FlavorReflector) Inject() (allReadyInjected bool) {
	if f.injected {
		return false
	}
	f.injected = true

	if f.FlavorLocal == nil {
		f.FlavorLocal = &local.FlavorLocal{}
	}
	f.FlavorLocal.Inject()

	if f.AllConnectorsFlavor == nil {
		f.AllConnectorsFlavor = &connectors.AllConnectorsFlavor{FlavorLocal: f.FlavorLocal}
	}
	f.AllConnectorsFlavor.Inject()

	f.PolicyReflector.Deps.PluginInfraDeps = *f.FlavorLocal.InfraDeps("policyreflector",
		local.WithConf())
	f.PolicyReflector.Deps.Publish = &f.AllConnectorsFlavor.ETCDDataSync
	f.PolicyReflector.Deps.Watch = &f.AllConnectorsFlavor.ETCDDataSync

	return true
}

// Plugins combines all plugins in the flavor into a slice.
func (f *FlavorReflector) Plugins() []*core.NamedPlugin {
	f.Inject()
	return core.ListPluginsInFlavor(f)
}
