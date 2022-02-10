#include <iostream>

#include <grpc/grpc.h>
#include <grpcpp/security/server_credentials.h>
#include <grpcpp/server.h>
#include <grpcpp/server_builder.h>
#include <grpcpp/server_context.h>
#include "proto/api/service/v1/metadata.grpc.pb.h"
#include "proto/api/service/v1/metadata.pb.h"
#include "proto/api/v1/robot.pb.h"
#include "proto/api/v1/robot.grpc.pb.h"

using grpc::Server;
using grpc::ServerBuilder;
using grpc::ServerContext;
using grpc::ServerReader;
using grpc::ServerReaderWriter;
using grpc::ServerWriter;
using proto::api::v1::RobotService;
using proto::api::service::v1::MetadataService;
using proto::api::v1::StatusRequest;
using proto::api::v1::StatusResponse;
using proto::api::v1::ConfigRequest;
using proto::api::v1::ConfigResponse;
using proto::api::service::v1::ResourcesRequest;
using proto::api::service::v1::ResourcesResponse;
using proto::api::service::v1::ResourceName;

class RobotServiceImpl final : public RobotService::Service {
 public:
 grpc::Status Config(ServerContext* context, const ConfigRequest* request, ConfigResponse* response) override{
return grpc::Status::OK;
 }
  grpc::Status Status(ServerContext* context, const StatusRequest* request, StatusResponse* response) override {
    (*response->mutable_status()->mutable_cameras())["camera"] = true;
    return grpc::Status::OK;
  }

};

class MetadataServiceImpl final : public MetadataService::Service  {
 public:
  grpc::Status Resources(ServerContext* context, const ResourcesRequest* request, ResourcesResponse* response) override{

    ResourceName* name = response->add_resources();    
    name->set_namespace_("rdk");
    name->set_type("component");
    name->set_subtype("camera");
    name->set_name("myCam");
    return grpc::Status::OK;
  }
};

int main(const int argc, const char** argv) {
  if (argc < 2) {
    std::cerr << "must supply grpc address" << std::endl;
    return 1;
  }
  RobotServiceImpl robotService;
  MetadataServiceImpl metadataService;
  ServerBuilder builder;
  builder.AddListeningPort(argv[1], grpc::InsecureServerCredentials());
  builder.RegisterService(&robotService);
  builder.RegisterService(&metadataService);
  std::unique_ptr<Server> server(builder.BuildAndStart());
  std::cout << "Server listening on " << argv[1] << std::endl;
  server->Wait();
  return 0;
}
