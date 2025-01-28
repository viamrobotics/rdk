# Tunnel
This example shows how to use the traffic tunneling feature in the viam-server


### Running
Run this example with `go run tunnel.go -addr {address of machine} -api-key {api key to use to connect to machine} -api-key-id {api key id to use to connect to machine} -dest {destination address to tunnel to (default 3389)} -src {source address to listen on (default 9090)}`

API key and API key id can be left blank if the machine is insecure.
