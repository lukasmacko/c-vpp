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

	r, err := c.Add(context.Background(), &cni.CNIRequest{})
	if err != nil {
		logroot.StandardLogger().Fatalf("could not receive response: %v", err)
	}
	logroot.StandardLogger().Printf("Response: %v (received from server)", r.Result)
}
