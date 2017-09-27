package main

import (
	"flag"

	"github.com/ligato/cn-infra/logging/logroot"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/contiv/contiv-vpp/plugins/contiv/model/cni"
)

const (
	defaultAddress = "localhost:9111"
)

var address = defaultAddress

func main() {
	flag.StringVar(&address, "address", defaultAddress, "address of GRPC server")
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
		ContainerId:      "sadjlfkj34l1kq4142348dw90",
		NetworkNamespace: "ns1",
		InterfaceName:    "eth0",
	}
	logroot.StandardLogger().WithField("req", req).Info("Sending request")

	r, err := c.Add(context.Background(), &cni.CNIRequest{})
	if err != nil {
		logroot.StandardLogger().Fatalf("could not receive response: %v", err)
	}
	logroot.StandardLogger().WithField("resp", *r).Infof("Response: %v (received from server)", r.Result)
	logroot.StandardLogger().Info("In order to test the connection run 'sudo ip netns exec ns1 ping 10.0.0.254'")
}
