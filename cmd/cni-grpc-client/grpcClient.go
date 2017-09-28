package main

import (
	"flag"

	"github.com/ligato/cn-infra/logging/logroot"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/contiv/contiv-vpp/plugins/contiv/model/cni"
)

const (
	defaultAddress          = "localhost:9111"
	defaultIfName           = "veth1"
	defaultNetworkNamespace = "/var/run/netns/55195f8d25bb4042"
	defaultContainerID      = "sadjlfkj34l1kq4142348dw90"
)

var (
	address     string
	ifname      string
	containerID string
	netns       string
)

func main() {
	flag.StringVar(&address, "address", defaultAddress, "address of GRPC server")
	flag.StringVar(&ifname, "ifname", defaultIfName, "interface name used in request")
	flag.StringVar(&containerID, "container-id", defaultContainerID, "container id used in request")
	flag.StringVar(&netns, "netns", defaultNetworkNamespace, "network namespace used in request")
	flag.Parse()

	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		logroot.StandardLogger().Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := cni.NewRemoteCNIClient(conn)

	req := cni.CNIRequest{
		Version:          "0.3.1",
		ContainerId:      containerID,
		NetworkNamespace: netns,
		InterfaceName:    ifname,
	}
	logroot.StandardLogger().WithField("req", req).Info("Sending request")

	r, err := c.Add(context.Background(), &req)
	if err != nil {
		logroot.StandardLogger().Fatalf("could not receive response: %v", err)
	}
	logroot.StandardLogger().WithField("resp", *r).Infof("Response: %v (received from server)", r.Result)
	logroot.StandardLogger().Info("In order to test the connection run 'sudo ip netns exec " + req.NetworkNamespace + " ping 10.0.0.254'")
}
