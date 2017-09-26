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

package main

import (
	"time"

	"github.com/ligato/cn-infra/core"

	"github.com/contiv/contiv-vpp/flavors/contiv"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logroot"
)

// init sets the default logging level
func init() {

	logroot.StandardLogger().SetLevel(logging.DebugLevel)
}

// Start Agent plugins selected for this example
func main() {

	flavor := contiv.FlavorContiv{}
	// Create new agent
	agentVar := core.NewAgent(logroot.StandardLogger(), 15*time.Second, flavor.Plugins()...)

	core.EventLoopWithInterrupt(agentVar, nil)
}
