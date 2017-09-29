package contiv

import (
	"github.com/contiv/contiv-vpp/plugins/contiv/model/cni"
	"github.com/ligato/cn-infra/logging/logroot"
	"github.com/onsi/gomega"
	"testing"
)

var req = cni.CNIRequest{
	Version:          "0.2.3",
	InterfaceName:    "eth0",
	ContainerId:      "sadfja813227wdhfjkh2319784dgh",
	NetworkNamespace: "/var/run/2345243",
}

func TestVeth1NameFromRequest(t *testing.T) {
	gomega.RegisterTestingT(t)

	server := newRemoteCNIServer(logroot.StandardLogger())

	hostIfName := server.veth1HostIfNameFromRequest(&req)
	gomega.Expect(hostIfName).To(gomega.BeEquivalentTo("eth0"))
}
