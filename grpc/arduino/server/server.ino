#include "src/utils/utils.h"
#include <Ethernet.h>
#include <vector>

#include "src/gen/pb_encode.h"
#include "src/gen/pb_decode.h"
#include "src/gen/robot.pb.h"
#include "src/http2/connection.h"
#include "src/http2/reset_stream_frame.h"
#include "src/http2/settings_frame.h"
#include "src/grpc/grpc_server.h"
#include "src/arduino/ethernet_client_transport.h"

using namespace std;

#define ARDUINO

byte MAC_ADDRESS[] = { 0xDE, 0xAD, 0xBE, 0xEF, 0xFE, 0xED };
uint16_t PORT = 8080;

proto_api_v1_StatusResponse dummyStatusResponse();
bool encodeStatusResponse(proto_api_v1_StatusResponse req, uint8_t** out, size_t* outLen);
proto_api_v1_CompassHeadingResponse dummyCompassHeadingResponse();
bool encodeCompassHeadingResponse(proto_api_v1_CompassHeadingResponse req, uint8_t** out, size_t* outLen);

EthernetServer server(PORT);
GRPCServer grpcServer = GRPCServer();
vector<Connection*> connections;

uint8_t status(uint8_t* data, uint32_t dataLen, uint8_t** response, uint32_t* responseLen) {
    uint8_t* buffer;
    size_t outLen;
    
    proto_api_v1_StatusResponse message = dummyStatusResponse();
    if (!encodeStatusResponse(message, &buffer, &outLen)) {
        debugPrint("encoding failed");
        return 1;
    }

    *response = buffer;
    *responseLen = outLen;
    return 0;
}

uint8_t compassHeading(uint8_t* data, uint32_t dataLen, uint8_t** response, uint32_t* responseLen) {
    // no validation of the request is done
    uint8_t* buffer;
    size_t outLen;
    
    proto_api_v1_CompassHeadingResponse message = dummyCompassHeadingResponse();
    if (!encodeCompassHeadingResponse(message, &buffer, &outLen)) {
        debugPrint("encoding failed");
        return 1;
    }

    *response = buffer;
    *responseLen = outLen;
    return 0;
}

void setup() {
    Serial.begin(9600);
    Ethernet.begin(MAC_ADDRESS);

    // Check for Ethernet hardware present
    if (Ethernet.hardwareStatus() == EthernetNoHardware) {
    debugPrint("Ethernet shield was not found.  Sorry, can't run without hardware. :(");
    while (true) {
        delay(1); // do nothing, no point running without Ethernet hardware
    }
    }
    if (Ethernet.linkStatus() == LinkOFF) {
        debugPrint("Ethernet cable is not connected.");
    }

    // start the server
    server.begin();
    debugPrint("server is at ");
    debugPrint(Ethernet.localIP());
    debugPrint(":");
    debugPrint(PORT);

    UnaryMethodHandler* statusHandler = new UnaryMethodHandler();
    statusHandler->name = "/proto.api.v1.RobotService/Status";
    statusHandler->handler = status;
    grpcServer.registerUnaryMethod(statusHandler);

    UnaryMethodHandler* compassHeadingHandler = new UnaryMethodHandler();
    compassHeadingHandler->name = "/proto.api.v1.RobotService/CompassHeading";
    compassHeadingHandler->handler = compassHeading;
    grpcServer.registerUnaryMethod(compassHeadingHandler);
}

void loop() {
    EthernetClient client = server.accept();
    for (auto it = connections.begin(); it != connections.end();) {
        Connection* conn = *it;
        if (conn->transport->connected()) {
            break;
        }
        grpcServer.removeConnection(conn);
        delete conn->transport;
        delete conn;
        it = connections.erase(it);
    }
    if (client) {
        debugPrint("new client");
        Connection* conn = new Connection(new EthernetClientTransport(client));
        connections.push_back(conn);
        grpcServer.addConnection(conn);
    } 
    grpcServer.runOnce();
    delay(1);
    return;
}

proto_api_v1_StatusResponse dummyStatusResponse() {
    proto_api_v1_StatusResponse message = proto_api_v1_StatusResponse_init_zero;
    proto_api_v1_Status status = proto_api_v1_Status_init_zero;
    status.sensors_count = 1;
    strncpy(status.sensors[0].key, "sensor1", sizeof(status.sensors[0].key));
    status.sensors[0].has_value = true;
    strncpy(status.sensors[0].value.type, "compass", sizeof(status.sensors[0].value.type));
    message.has_status = true;
    message.status = status;
    return message;
}

bool encodeStatusResponse(proto_api_v1_StatusResponse resp, uint8_t** out, size_t* outLen) {
    size_t bufSize = 1024*sizeof(uint8_t);
    *out = new uint8_t[bufSize];
    pb_ostream_t stream = pb_ostream_from_buffer(*out, bufSize);
    bool status = pb_encode(&stream, proto_api_v1_StatusResponse_fields, &resp);
    *outLen = stream.bytes_written;
    return status;
}

proto_api_v1_CompassHeadingResponse dummyCompassHeadingResponse() {
    proto_api_v1_CompassHeadingResponse message = proto_api_v1_CompassHeadingResponse_init_zero;
    message.heading = 5.4;
    return message;
}

bool encodeCompassHeadingResponse(proto_api_v1_CompassHeadingResponse resp, uint8_t** out, size_t* outLen) {
    size_t bufSize = 1024*sizeof(uint8_t);
    *out = new uint8_t[bufSize];
    pb_ostream_t stream = pb_ostream_from_buffer(*out, bufSize);
    bool status = pb_encode(&stream, proto_api_v1_CompassHeadingResponse_fields, &resp);
    *outLen = stream.bytes_written;
    return status;
}

