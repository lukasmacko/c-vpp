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
	"encoding/json"
	"io/ioutil"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

func cmdAdd(args *skel.CmdArgs) error {
	// TODO: CALL AGENT
	cniJson, _ := json.Marshal(args)
	ioutil.WriteFile("/tmp/cni-add.json", cniJson, 0644)

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

	//_, ipnet, _ := net.ParseCIDR("192.168.53.53/24")
	//ip, _, _ := net.ParseCIDR("192.168.53.1/24")
	//ifidx := 0

	result := &current.Result{
	//CNIVersion: "0.2.0",
	//Interfaces: []*current.Interface{
	//	{
	//		Name: "lo",
	//		// mac, sandbox?
	//	},
	//},
	//IPs: []*current.IPConfig{
	//	{
	//		Version:   "4",
	//		Interface: &ifidx,
	//		Address:   *ipnet,
	//		Gateway:   ip,
	//	},
	//},
	}
	return result.Print()
}

func cmdDel(args *skel.CmdArgs) error {
	// TODO: CALL AGENT
	cniJson, _ := json.Marshal(args)
	ioutil.WriteFile("/tmp/cni-del.json", cniJson, 0644)

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

	return nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdDel, version.All)
}
