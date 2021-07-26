#include <iostream>

#include <grpc/grpc.h>
#include <grpcpp/channel.h>
#include <grpcpp/client_context.h>
#include <grpcpp/create_channel.h>
#include <google/protobuf/util/json_util.h>
#include <grpcpp/security/credentials.h>

#include "proto/api/v1/robot.pb.h"
#include "proto/api/v1/robot.grpc.pb.h"

using grpc::Channel;
using grpc::ClientContext;
using grpc::Status;
using proto::api::v1::RobotService;
using proto::api::v1::StatusRequest;
using proto::api::v1::StatusResponse;

int main(const int argc, const char** argv) {
  if (argc < 2) {
    std::cerr << "must supply grpc address" << std::endl;
    return 1;
  }
  const std::shared_ptr<Channel> channel = grpc::CreateChannel(argv[1], grpc::InsecureChannelCredentials());
  const std::unique_ptr<RobotService::Stub> client = RobotService::NewStub(channel);
  ClientContext context;
  const StatusRequest request;
  StatusResponse response;
  const Status gStatus = client->Status(&context, request, &response);
  if (!gStatus.ok()) {
    std::cout << "Status rpc failed." << std::endl;
    return 1;
  }
  std::string json;
  google::protobuf::util::MessageToJsonString(response, &json);
  std::cout << json << std::endl;
  return 0;
}
