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

package contiv

import (
	"github.com/contiv/contiv-vpp/plugins/contiv/model/cni"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/clientv1/defaultplugins/localclient"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/ifplugin/model/interfaces"
	"golang.org/x/net/context"
)

type remoteCNIserver struct {
	logging.Logger
}

func newRemoteCNIServer(logger logging.Logger) *remoteCNIserver {
	return &remoteCNIserver{logger}
}

func (s *remoteCNIserver) Add(ctx context.Context, request *cni.CNIRequest) (*cni.CNIReply, error) {
	s.Info("Add request received ", *request)
	err := s.configureContainerConnectivity()
	if err != nil {
		return &cni.CNIReply{Result: 1, Error: err.Error()}, nil
	}
	return &cni.CNIReply{}, nil
}

// The request to delete a container from network.
func (s *remoteCNIserver) Delete(ctx context.Context, request *cni.CNIRequest) (*cni.CNIReply, error) {
	s.Info("Delete request received ", *request)
	return &cni.CNIReply{}, nil
}

func (s *remoteCNIserver) configureContainerConnectivity() error {
	return localclient.DataChangeRequest("CNI").
		Put().
		Interface(&memif1AsMaster).
		Send().ReceiveReply()
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
