### gRPC CNI Plugin

This CNI plugin forwards the CNI requests to the specified gRPC server.

To run the plugin for testing purposes, create the file `/etc/cni/net.d/10-contiv-cni.conf`:
```
{
	"cniVersion": "0.2.0",
	"type": "contiv-cni",
	"grpcServer": "localhost:9111"
}
```

Given that the `contiv-cni` binary exists in the folder 
`$GOPATH/src/github.com/contiv/contiv-vpp/cmd/contiv-cni`: 

Set `CNI_PATH` environment variable:
```
CNI_PATH=$GOPATH/src/github.com/contiv/contiv-vpp/cmd/contiv-cni
```

Enter the folder with CNI scripts and execute the following:
```
cd vendor/github.com/containernetworking/cni/scripts
sudo CNI_PATH=$CNI_PATH ./priv-net-run.sh ifconfig
```
