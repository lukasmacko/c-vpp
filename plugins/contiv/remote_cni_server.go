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
	"github.com/ligato/vpp-agent/clientv1/linux/localclient"
	vpp_intf "github.com/ligato/vpp-agent/plugins/defaultplugins/ifplugin/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/l2plugin/model/l2"
	linux_intf "github.com/ligato/vpp-agent/plugins/linuxplugin/model/interfaces"

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

// configureContainerConnectivity creates veth pair where
// one end is ns1 namespace, the other is in default namespace.
// the end in default namespace is connected to VPP using afpacket.

func (s *remoteCNIserver) configureContainerConnectivity() error {
	return localclient.DataChangeRequest("CNI").
		Put().
		LinuxInterface(&veth11).
		LinuxInterface(&veth12).
		VppInterface(&afpacket1).
		VppInterface(&loop1).
		BD(&bd).
		Send().ReceiveReply()
}

var veth11 = linux_intf.LinuxInterfaces_Interface{
	Name:    "veth11",
	Type:    linux_intf.LinuxInterfaces_VETH,
	Enabled: true,
	Veth: &linux_intf.LinuxInterfaces_Interface_Veth{
		PeerIfName: "veth12",
	},
	IpAddresses: []string{"10.0.0.1/24"},
	Namespace: &linux_intf.LinuxInterfaces_Interface_Namespace{
		Type: linux_intf.LinuxInterfaces_Interface_Namespace_NAMED_NS,
		Name: "ns1",
	},
}

var veth12 = linux_intf.LinuxInterfaces_Interface{
	Name:    "veth12",
	Type:    linux_intf.LinuxInterfaces_VETH,
	Enabled: true,
	Veth: &linux_intf.LinuxInterfaces_Interface_Veth{
		PeerIfName: "veth11",
	},
}

var afpacket1 = vpp_intf.Interfaces_Interface{
	Name:    "afpacket1",
	Type:    vpp_intf.InterfaceType_AF_PACKET_INTERFACE,
	Enabled: true,
	Afpacket: &vpp_intf.Interfaces_Interface_Afpacket{
		HostIfName: "veth12",
	},
}

var bd = l2.BridgeDomains_BridgeDomain{
	Name:                "br1",
	Flood:               true,
	UnknownUnicastFlood: true,
	Forward:             true,
	Learn:               true,
	ArpTermination:      false,
	MacAge:              0, /* means disable aging */
	Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
		{
			Name: "afpacket1",
			BridgedVirtualInterface: false,
		}, {
			Name: "loop1",
			BridgedVirtualInterface: true,
		},
	},
}

var loop1 = vpp_intf.Interfaces_Interface{
	Name:        "loop1",
	Enabled:     true,
	IpAddresses: []string{"10.0.0.2/24"},
	Type:        vpp_intf.InterfaceType_SOFTWARE_LOOPBACK,
}
