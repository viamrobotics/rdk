const { EchoRequest } = require('proto/rpc/examples/echo/v1/echo_pb.js');
const { EchoServicePromiseClient } = require('proto/rpc/examples/echo/v1/echo_grpc_web_pb.js');

const echoService = new EchoServicePromiseClient('http://localhost:8080');

async function echo(message) {
	var request = new EchoRequest();
	request.setMessage(message);
	return (await echoService.echo(request)).toObject();
}

echo("hey!").then(console.log).catch(console.log);
