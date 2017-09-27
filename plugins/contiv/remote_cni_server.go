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

	"github.com/prometheus/common/log"
	"golang.org/x/net/context"
	"strconv"
	"sync"
)

type remoteCNIserver struct {
	logging.Logger

	sync.Mutex
	bdCreated bool
	counter   int
}

const (
	resultOk  uint32 = 0
	resultErr uint32 = 1
)

func newRemoteCNIServer(logger logging.Logger) *remoteCNIserver {
	return &remoteCNIserver{Logger: logger}
}

func (s *remoteCNIserver) Add(ctx context.Context, request *cni.CNIRequest) (*cni.CNIReply, error) {
	s.Info("Add request received ", *request)
	return s.configureContainerConnectivity(request)
}

// The request to delete a container from network.
func (s *remoteCNIserver) Delete(ctx context.Context, request *cni.CNIRequest) (*cni.CNIReply, error) {
	s.Info("Delete request received ", *request)
	return &cni.CNIReply{}, nil
}

// configureContainerConnectivity creates veth pair where
// one end is ns1 namespace, the other is in default namespace.
// the end in default namespace is connected to VPP using afpacket.
func (s *remoteCNIserver) configureContainerConnectivity(request *cni.CNIRequest) (*cni.CNIReply, error) {
	s.Lock()
	defer s.Unlock()

	s.counter++

	veth1 := s.veth1FromRequest(request)
	veth2 := s.veth2FromRequest(request)
	afpacket := s.afpacketFromRequest(request)
	bd := s.bridgeDomain()
	log.Info("Bridge domain", *bd)

	txn := localclient.DataChangeRequest("CNI").
		Put().
		LinuxInterface(veth1).
		LinuxInterface(veth2).
		VppInterface(afpacket)

	if !s.bdCreated {
		txn.VppInterface(s.bviInterface())
	}

	err := txn.BD(bd).
		Send().ReceiveReply()

	res := resultOk
	if err == nil {
		s.bdCreated = true
	} else {
		res = resultErr
	}

	reply := &cni.CNIReply{
		Result: res,
	}
	return reply, err
}

//
// +-------------------------------------------------+
// |                                                 |
// |                         +----------------+      |
// |                         |     Loop1      |      |
// |      Bridge domain      |     BVI        |      |
// |                         +----------------+      |
// |    +------+       +------+                      |
// |    |  AF1 |       | AFn  |                      |
// |    |      |  ...  |      |                      |
// |    +------+       +------+                      |
// |      ^                                          |
// |      |                                          |
// +------|------------------------------------------+
//        v
// +------------+
// |            |
// | Veth12     |
// |            |
// +------------+
//        ^
//        |
// +------|------------+
// |  NS1 v            |
// |  +------------+   |
// |  |            |   |
// |  | Veth11     |   |
// |  |            |   |
// |  +------------+   |
// |                   |
// +-------------------+

func (s *remoteCNIserver) veth1FromRequest(request *cni.CNIRequest) *linux_intf.LinuxInterfaces_Interface {
	var veth11 = linux_intf.LinuxInterfaces_Interface{
		Name:    "veth" + strconv.Itoa(s.counter) + "1",
		Type:    linux_intf.LinuxInterfaces_VETH,
		Enabled: true,
		Veth: &linux_intf.LinuxInterfaces_Interface_Veth{
			PeerIfName: "veth" + strconv.Itoa(s.counter) + "2",
		},
		IpAddresses: []string{"10.0.0." + strconv.Itoa(s.counter) + "/24"},
		Namespace: &linux_intf.LinuxInterfaces_Interface_Namespace{
			Type: linux_intf.LinuxInterfaces_Interface_Namespace_NAMED_NS,
			Name: "ns" + strconv.Itoa(s.counter),
		},
	}
	return &veth11
}

func (s *remoteCNIserver) veth2FromRequest(request *cni.CNIRequest) *linux_intf.LinuxInterfaces_Interface {
	var veth12 = linux_intf.LinuxInterfaces_Interface{
		Name:    "veth" + strconv.Itoa(s.counter) + "2",
		Type:    linux_intf.LinuxInterfaces_VETH,
		Enabled: true,
		Veth: &linux_intf.LinuxInterfaces_Interface_Veth{
			PeerIfName: "veth" + strconv.Itoa(s.counter) + "1",
		},
	}
	return &veth12
}

func (s *remoteCNIserver) afpacketFromRequest(request *cni.CNIRequest) *vpp_intf.Interfaces_Interface {
	var afpacket = vpp_intf.Interfaces_Interface{
		Name:    "afpacket" + strconv.Itoa(s.counter),
		Type:    vpp_intf.InterfaceType_AF_PACKET_INTERFACE,
		Enabled: true,
		Afpacket: &vpp_intf.Interfaces_Interface_Afpacket{
			HostIfName: "veth" + strconv.Itoa(s.counter) + "2",
		},
	}
	return &afpacket
}

func (s *remoteCNIserver) bridgeDomain() *l2.BridgeDomains_BridgeDomain {
	var ifs = []*l2.BridgeDomains_BridgeDomain_Interfaces{
		{
			Name: "loop1",
			BridgedVirtualInterface: true,
		}}

	for i := 1; i <= s.counter; i++ {
		ifs = append(ifs, &l2.BridgeDomains_BridgeDomain_Interfaces{
			Name: "afpacket" + strconv.Itoa(i),
			BridgedVirtualInterface: false,
		})
	}

	var bd = l2.BridgeDomains_BridgeDomain{
		Name:                "br1",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		ArpTermination:      false,
		MacAge:              0, /* means disable aging */
		Interfaces:          ifs,
	}
	return &bd
}

func (s *remoteCNIserver) bviInterface() *vpp_intf.Interfaces_Interface {
	var loop1 = vpp_intf.Interfaces_Interface{
		Name:        "loop1",
		Enabled:     true,
		IpAddresses: []string{"10.0.0.254/24"},
		Type:        vpp_intf.InterfaceType_SOFTWARE_LOOPBACK,
	}
	return &loop1
}
