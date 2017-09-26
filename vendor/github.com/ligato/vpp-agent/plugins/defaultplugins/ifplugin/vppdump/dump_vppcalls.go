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

package vppdump

import (
	"bytes"
	"fmt"
	"net"
	"strings"

	govppapi "git.fd.io/govpp.git/api"
	log "github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/ifplugin/bin_api/interfaces"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/ifplugin/bin_api/ip"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/ifplugin/bin_api/memif"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/ifplugin/bin_api/tap"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/ifplugin/bin_api/vxlan"
	ifnb "github.com/ligato/vpp-agent/plugins/defaultplugins/ifplugin/model/interfaces"
)

// Interface is the wrapper structure for the interface northbound API structure.
type Interface struct {
	VPPInternalName string
	ifnb.Interfaces_Interface
}

// DumpInterfaces dumps VPP interface data into the northbound API data structure
// map indexed by software interface index.
//
// LIMITATIONS:
// - there is no af_packet dump binary API. We relay on naming conventions of the internal VPP interface names
// - ip.IPAddressDetails has wrong internal structure, as a workaround we need to handle them as notifications
//
func DumpInterfaces(vppChan *govppapi.Channel) (map[uint32]*Interface, error) {

	// map for the resulting interfaces
	ifs := make(map[uint32]*Interface)

	// first, dump all interfaces to create initial data
	reqCtx := vppChan.SendMultiRequest(&interfaces.SwInterfaceDump{})

	for {
		ifDetails := &interfaces.SwInterfaceDetails{}
		stop, err := reqCtx.ReceiveReply(ifDetails)
		if stop {
			break // break out of the loop
		}
		if err != nil {
			log.DefaultLogger().Error(err)
			return nil, err
		}

		iface := &Interface{
			VPPInternalName: string(bytes.Trim(ifDetails.InterfaceName, "\x00")),
			Interfaces_Interface: ifnb.Interfaces_Interface{
				Type:        guessInterfaceType(string(ifDetails.InterfaceName)), // the type may be amended later by further dumps
				Enabled:     ifDetails.AdminUpDown > 0,
				PhysAddress: net.HardwareAddr(ifDetails.L2Address[:ifDetails.L2AddressLength]).String(),
			},
		}
		ifs[ifDetails.SwIfIndex] = iface

		if iface.Type == ifnb.InterfaceType_AF_PACKET_INTERFACE {
			err := dumpAFPacketDetails(ifs, ifDetails.SwIfIndex, iface.VPPInternalName)
			if err != nil {
				return nil, err
			}
		}
	}

	err := dumpIPAddressDetails(vppChan, ifs, 0)
	if err != nil {
		return nil, err
	}

	err = dumpIPAddressDetails(vppChan, ifs, 1)
	if err != nil {
		return nil, err
	}

	err = dumpMemifDetails(vppChan, ifs)
	if err != nil {
		return nil, err
	}

	err = dumpTapDetails(vppChan, ifs)
	if err != nil {
		return nil, err
	}

	err = dumpVxlanDetails(vppChan, ifs)
	if err != nil {
		return nil, err
	}

	return ifs, nil
}

// dumpIPAddressDetails dumps IP address details of interfaces from VPP and fills them into the provided interface map.
func dumpIPAddressDetails(vppChan *govppapi.Channel, ifs map[uint32]*Interface, isIPv6 uint8) error {

	// TODO: workaround for incorrect ip.IPAddressDetails message
	notifChan := make(chan govppapi.Message, 100)
	subs, _ := vppChan.SubscribeNotification(notifChan, ip.NewIPAddressDetails)

	// dump IP addresses of each interface
	for idx := range ifs {
		reqCtx := vppChan.SendMultiRequest(&ip.IPAddressDump{SwIfIndex: idx, IsIpv6: isIPv6})
		for {
			ipDetails := &ip.IPAddressDetails{}
			stop, err := reqCtx.ReceiveReply(ipDetails)
			if stop {
				break // break out of the loop
			}
			if err != nil {
				log.DefaultLogger().Error(err)
				return err
			}
			processIPDetails(ifs, ipDetails)
		}

		// TODO: workaround for incorrect ip.IPAddressDetails message
		for len(notifChan) > 0 {
			notifMsg := <-notifChan
			processIPDetails(ifs, notifMsg.(*ip.IPAddressDetails))
		}
	}

	// TODO: workaround for incorrect ip.IPAddressDetails message
	vppChan.UnsubscribeNotification(subs)

	return nil
}

// processIPDetails processes ip.IPAddressDetails binary API message and fills the details into the provided interface map.
func processIPDetails(ifs map[uint32]*Interface, ipDetails *ip.IPAddressDetails) {
	if ifs[ipDetails.SwIfIndex].IpAddresses == nil {
		ifs[ipDetails.SwIfIndex].IpAddresses = make([]string, 0)
	}
	var ip string
	if ipDetails.IsIpv6 == 1 {
		ip = fmt.Sprintf("%s/%d", net.IP(ipDetails.IP).To16().String(), uint32(ipDetails.PrefixLength))
	} else {
		ip = fmt.Sprintf("%s/%d", net.IP(ipDetails.IP[:4]).To4().String(), uint32(ipDetails.PrefixLength))
	}
	ifs[ipDetails.SwIfIndex].IpAddresses = append(ifs[ipDetails.SwIfIndex].IpAddresses, ip)
}

// dumpAFPacketDetails fills af_packet interface details into the provided interface map.
func dumpAFPacketDetails(ifs map[uint32]*Interface, swIfIndex uint32, ifName string) error {
	ifs[swIfIndex].Afpacket = &ifnb.Interfaces_Interface_Afpacket{
		HostIfName: strings.TrimPrefix(ifName, "host-"),
	}
	ifs[swIfIndex].Type = ifnb.InterfaceType_AF_PACKET_INTERFACE
	return nil
}

// dumpMemifDetails dumps memif interface details from VPP and fills them into the provided interface map.
func dumpMemifDetails(vppChan *govppapi.Channel, ifs map[uint32]*Interface) error {
	reqCtx := vppChan.SendMultiRequest(&memif.MemifDump{})
	for {
		memifDetails := &memif.MemifDetails{}
		stop, err := reqCtx.ReceiveReply(memifDetails)
		if stop {
			break // break out of the loop
		}
		if err != nil {
			log.DefaultLogger().Error(err)
			return err
		}
		ifs[memifDetails.SwIfIndex].Memif = &ifnb.Interfaces_Interface_Memif{
			Master: memifDetails.Role == 0,
			Mode:   memifModetoNB(memifDetails.Mode),
			Id:     memifDetails.ID,
			//TODO Secret - not available in the binary API
			SocketFilename: string(bytes.Trim(memifDetails.SocketFilename, "\x00")),
			RingSize:       memifDetails.RingSize,
			BufferSize:     uint32(memifDetails.BufferSize),
			// TODO: RxQueues, TxQueues - not available in the binary API
		}
		ifs[memifDetails.SwIfIndex].Type = ifnb.InterfaceType_MEMORY_INTERFACE
	}
	return nil
}

// dumpTapDetails dumps tap interface details from VPP and fills them into the provided interface map.
func dumpTapDetails(vppChan *govppapi.Channel, ifs map[uint32]*Interface) error {
	reqCtx := vppChan.SendMultiRequest(&tap.SwInterfaceTapDump{})
	for {
		tapDetails := &tap.SwInterfaceTapDetails{}
		stop, err := reqCtx.ReceiveReply(tapDetails)
		if stop {
			break // break out of the loop
		}
		if err != nil {
			log.DefaultLogger().Error(err)
			return err
		}
		ifs[tapDetails.SwIfIndex].Tap = &ifnb.Interfaces_Interface_Tap{
			HostIfName: string(bytes.Trim(tapDetails.DevName, "\x00")),
		}
		ifs[tapDetails.SwIfIndex].Type = ifnb.InterfaceType_TAP_INTERFACE
	}
	return nil
}

// dumpVxlanDetails dumps VXLAN interface details from VPP and fills them into the provided interface map.
func dumpVxlanDetails(vppChan *govppapi.Channel, ifs map[uint32]*Interface) error {
	reqCtx := vppChan.SendMultiRequest(&vxlan.VxlanTunnelDump{SwIfIndex: ^uint32(0)})
	for {
		vxlanDetails := &vxlan.VxlanTunnelDetails{}
		stop, err := reqCtx.ReceiveReply(vxlanDetails)
		if stop {
			break // break out of the loop
		}
		if err != nil {
			log.DefaultLogger().Error(err)
			return err
		}
		if vxlanDetails.IsIpv6 == 1 {
			ifs[vxlanDetails.SwIfIndex].Vxlan = &ifnb.Interfaces_Interface_Vxlan{
				SrcAddress: net.IP(vxlanDetails.SrcAddress).To16().String(),
				DstAddress: net.IP(vxlanDetails.DstAddress).To16().String(),
				Vni:        vxlanDetails.Vni,
			}
		} else {
			ifs[vxlanDetails.SwIfIndex].Vxlan = &ifnb.Interfaces_Interface_Vxlan{
				SrcAddress: net.IP(vxlanDetails.SrcAddress[:4]).To4().String(),
				DstAddress: net.IP(vxlanDetails.DstAddress[:4]).To4().String(),
				Vni:        vxlanDetails.Vni,
			}
		}
		ifs[vxlanDetails.SwIfIndex].Type = ifnb.InterfaceType_VXLAN_TUNNEL
	}
	return nil
}

// guessInterfaceType attempts to guess the correct interface type from its internal name (as given by VPP).
// This is required mainly for those interface types, that do not provide dump binary API,
// such as loopback of af_packet.
func guessInterfaceType(ifName string) ifnb.InterfaceType {
	switch {
	case strings.HasPrefix(ifName, "loop"):
		return ifnb.InterfaceType_SOFTWARE_LOOPBACK
	case strings.HasPrefix(ifName, "local"):
		return ifnb.InterfaceType_SOFTWARE_LOOPBACK
	case strings.HasPrefix(ifName, "memif"):
		return ifnb.InterfaceType_MEMORY_INTERFACE
	case strings.HasPrefix(ifName, "tap"):
		return ifnb.InterfaceType_TAP_INTERFACE
	case strings.HasPrefix(ifName, "host"):
		return ifnb.InterfaceType_AF_PACKET_INTERFACE
	case strings.HasPrefix(ifName, "vxlan"):
		return ifnb.InterfaceType_VXLAN_TUNNEL
	}
	return ifnb.InterfaceType_ETHERNET_CSMACD
}

// memifModetoNB converts binary API type of memif mode to the northbound API type memif mode.
func memifModetoNB(mode uint8) ifnb.Interfaces_Interface_Memif_MemifMode {
	switch mode {
	case 0:
		return ifnb.Interfaces_Interface_Memif_ETHERNET
	case 1:
		return ifnb.Interfaces_Interface_Memif_IP
	case 2:
		return ifnb.Interfaces_Interface_Memif_PUNT_INJECT
	}
	return ifnb.Interfaces_Interface_Memif_ETHERNET
}
