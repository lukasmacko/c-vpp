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
	"context"
	"net"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/contiv/contiv-vpp/plugins/contiv/model/cni"
	"github.com/ligato/cn-infra/logging/logroot"
	"github.com/vishvananda/netlink"
	"google.golang.org/grpc"
)

const (
	defaultAddress = "localhost:9111"
)

func cmdAdd(args *skel.CmdArgs) error {

	// TODO: to be removed, this adds a loopback - for debug purposes
	args.IfName = "lo" // ignore config, this only works for loopback
	err := ns.WithNetNSPath(args.Netns, func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(args.IfName)
		if err != nil {
			return err // not tested
		}

		err = netlink.LinkSetUp(link)
		if err != nil {
			return err // not tested
		}

		return nil
	})
	if err != nil {
		return err // not tested
	}

	c := getRemoteCNIClient()
	r, err := c.Add(context.Background(), &cni.CNIRequest{})
	if err != nil {
		return err
	}

	result := &current.Result{
		CNIVersion: "0.3.1",
	}
	for ifidx, iface := range r.Interfaces {
		// append interface info
		result.Interfaces = append(result.Interfaces, &current.Interface{
			Name:    iface.Name,
			Mac:     iface.Mac,
			Sandbox: iface.Sandbox,
		})
		for _, ip := range iface.IpAddresses {
			// append interface ip address info
			_, ipAddr, err := net.ParseCIDR(ip.Address)
			if err != nil {
				return err
			}
			gwAddr, _, err := net.ParseCIDR(ip.Gateway)
			if err != nil {
				return err
			}
			version := "4"
			if ip.Version == cni.CNIReply_Interface_IP_IPV6 {
				version = "6"
			}
			result.IPs = append(result.IPs, &current.IPConfig{
				Address:   *ipAddr,
				Version:   version,
				Interface: &ifidx,
				Gateway:   gwAddr,
			})
		}
	}

	return result.Print()
}

func cmdDel(args *skel.CmdArgs) error {

	// TODO: to be removed, this removes a loopback - for debug purposes
	args.IfName = "lo" // ignore config, this only works for loopback
	err := ns.WithNetNSPath(args.Netns, func(ns.NetNS) error {
		link, err := netlink.LinkByName(args.IfName)
		if err != nil {
			return err // not tested
		}

		err = netlink.LinkSetDown(link)
		if err != nil {
			return err // not tested
		}

		return nil
	})
	if err != nil {
		return err // not tested
	}

	// TODO: call grpc

	return nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdDel, version.All)
}

func getRemoteCNIClient() cni.RemoteCNIClient {
	// Set up a connection to the server.
	conn, err := grpc.Dial(defaultAddress, grpc.WithInsecure()) // TODO: parse from plugin config
	if err != nil {
		logroot.StandardLogger().Fatalf("did not connect: %v", err)
	}
	//defer conn.Close() // TODO close properly
	return cni.NewRemoteCNIClient(conn)
}
