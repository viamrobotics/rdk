#include <iostream>

#include <grpc/grpc.h>
#include <grpcpp/security/server_credentials.h>
#include <grpcpp/server.h>
#include <grpcpp/server_builder.h>
#include <grpcpp/server_context.h>

#include "proto/api/v1/robot.pb.h"
#include "proto/api/v1/robot.grpc.pb.h"

using grpc::Server;
using grpc::ServerBuilder;
using grpc::ServerContext;
using grpc::ServerReader;
using grpc::ServerReaderWriter;
using grpc::ServerWriter;
using proto::api::v1::RobotService;
using proto::api::v1::StatusRequest;
using proto::api::v1::StatusResponse;

class RobotServiceImpl final : public RobotService::Service {
 public:
  grpc::Status Status(ServerContext* context, const StatusRequest* request, StatusResponse* response) override {
    (*response->mutable_status()->mutable_bases())["base1"] = true;
    return grpc::Status::OK;
  }
};

int main(const int argc, const char** argv) {
  if (argc < 2) {
    std::cerr << "must supply grpc address" << std::endl;
    return 1;
  }
  RobotServiceImpl service;
  ServerBuilder builder;
  builder.AddListeningPort(argv[1], grpc::InsecureServerCredentials());
  builder.RegisterService(&service);
  std::unique_ptr<Server> server(builder.BuildAndStart());
  std::cout << "Server listening on " << argv[1] << std::endl;
  server->Wait();
  return 0;
}
