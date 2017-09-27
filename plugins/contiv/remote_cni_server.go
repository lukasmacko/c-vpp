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

	// bdCreated is true if the bridge domain on the vpp for apackets is configured
	bdCreated bool
	// counter of connected containers. It is used for generating afpacket names
	// and assigned ip addresses.
	counter int
	// created afPacket that are in the bridge domain
	// map is used to support quick removal
	afPackets map[string]interface{}
}

const (
	resultOk       uint32 = 0
	resultErr      uint32 = 1
	vethNameMaxLen        = 15
	bdName                = "bd1"
	bviName               = "loop1"
	ipMask                = "24"
	ipPrefix              = "10.0.0"
	bviIP                 = ipPrefix + ".254/" + ipMask
)

func newRemoteCNIServer(logger logging.Logger) *remoteCNIserver {
	return &remoteCNIserver{Logger: logger, afPackets: map[string]interface{}{}}
}

// Add connects the container to the network.
func (s *remoteCNIserver) Add(ctx context.Context, request *cni.CNIRequest) (*cni.CNIReply, error) {
	s.Info("Add request received ", *request)
	return s.configureContainerConnectivity(request)
}

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

	log.Info("veth1", veth1)
	log.Info("veth2", veth2)
	log.Info("afpacket", afpacket)

	// create entry in the afpacket map => add afpacket into bridge domain
	s.afPackets[afpacket.Name] = nil

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
	errMsg := ""
	if err == nil {
		s.bdCreated = true
	} else {
		res = resultErr
		errMsg = err.Error()
		delete(s.afPackets, afpacket.Name)
	}

	reply := &cni.CNIReply{
		Result: res,
		Error:  errMsg,
		Interfaces: []*cni.CNIReply_Interface{
			{
				Name:    veth1.Name,
				Sandbox: veth1.Namespace.Name,
				IpAddresses: []*cni.CNIReply_Interface_IP{
					{
						Version: cni.CNIReply_Interface_IP_IPV4,
						Address: veth1.IpAddresses[0],
					},
				},
			},
		},
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
// | Veth2      |
// |            |
// +------------+
//        ^
//        |
// +------|------------+
// |  NS1 v            |
// |  +------------+   |
// |  |            |   |
// |  | Veth1      |   |
// |  |            |   |
// |  +------------+   |
// |                   |
// +-------------------+

func (s *remoteCNIserver) veth1NameFromRequest(request *cni.CNIRequest) string {
	return request.InterfaceName
}

func (s *remoteCNIserver) veth2NameFromRequest(request *cni.CNIRequest) string {
	if len(request.ContainerId) > vethNameMaxLen {
		return request.ContainerId[:vethNameMaxLen]
	}
	return request.ContainerId
}

func (s *remoteCNIserver) ipAddrForContainer() string {
	return ipPrefix + "." + strconv.Itoa(s.counter) + "/" + ipMask
}

func (s *remoteCNIserver) veth1FromRequest(request *cni.CNIRequest) *linux_intf.LinuxInterfaces_Interface {
	return &linux_intf.LinuxInterfaces_Interface{
		Name:    s.veth1NameFromRequest(request),
		Type:    linux_intf.LinuxInterfaces_VETH,
		Enabled: true,
		Veth: &linux_intf.LinuxInterfaces_Interface_Veth{
			PeerIfName: s.veth2NameFromRequest(request),
		},
		IpAddresses: []string{s.ipAddrForContainer()},
		Namespace: &linux_intf.LinuxInterfaces_Interface_Namespace{
			Type:     linux_intf.LinuxInterfaces_Interface_Namespace_FILE_REF_NS,
			Filepath: request.NetworkNamespace,
		},
	}
}

func (s *remoteCNIserver) veth2FromRequest(request *cni.CNIRequest) *linux_intf.LinuxInterfaces_Interface {
	return &linux_intf.LinuxInterfaces_Interface{
		Name:    s.veth2NameFromRequest(request),
		Type:    linux_intf.LinuxInterfaces_VETH,
		Enabled: true,
		Veth: &linux_intf.LinuxInterfaces_Interface_Veth{
			PeerIfName: s.veth1NameFromRequest(request),
		},
	}
}

func (s *remoteCNIserver) afpacketFromRequest(request *cni.CNIRequest) *vpp_intf.Interfaces_Interface {
	return &vpp_intf.Interfaces_Interface{
		Name:    "afpacket" + strconv.Itoa(s.counter),
		Type:    vpp_intf.InterfaceType_AF_PACKET_INTERFACE,
		Enabled: true,
		Afpacket: &vpp_intf.Interfaces_Interface_Afpacket{
			HostIfName: s.veth2NameFromRequest(request),
		},
	}
}

func (s *remoteCNIserver) bridgeDomain() *l2.BridgeDomains_BridgeDomain {
	var ifs = []*l2.BridgeDomains_BridgeDomain_Interfaces{
		{
			Name: bviName,
			BridgedVirtualInterface: true,
		}}

	for af := range s.afPackets {
		ifs = append(ifs, &l2.BridgeDomains_BridgeDomain_Interfaces{
			Name: af,
			BridgedVirtualInterface: false,
		})
	}

	return &l2.BridgeDomains_BridgeDomain{
		Name:                bdName,
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		ArpTermination:      false,
		MacAge:              0, /* means disable aging */
		Interfaces:          ifs,
	}
}

func (s *remoteCNIserver) bviInterface() *vpp_intf.Interfaces_Interface {
	return &vpp_intf.Interfaces_Interface{
		Name:        bviName,
		Enabled:     true,
		IpAddresses: []string{bviIP},
		Type:        vpp_intf.InterfaceType_SOFTWARE_LOOPBACK,
	}
}
