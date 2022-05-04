#include <iostream>

#include <grpc/grpc.h>
#include <grpcpp/security/server_credentials.h>
#include <grpcpp/server.h>
#include <grpcpp/server_builder.h>
#include <grpcpp/server_context.h>

#include "proto/api/common/v1/common.grpc.pb.h"
#include "proto/api/common/v1/common.pb.h"
#include "proto/api/component/camera/v1/camera.grpc.pb.h"
#include "proto/api/component/camera/v1/camera.pb.h"
#include "proto/api/service/metadata/v1/metadata.grpc.pb.h"
#include "proto/api/service/metadata/v1/metadata.pb.h"

using grpc::Server;
using grpc::ServerBuilder;
using grpc::ServerContext;
using grpc::ServerReader;
using grpc::ServerReaderWriter;
using grpc::ServerWriter;
using proto::api::common::v1::ResourceName;
using proto::api::component::camera::v1::CameraService;
using proto::api::component::camera::v1::GetFrameRequest;
using proto::api::component::camera::v1::GetFrameResponse;
using proto::api::component::camera::v1::GetPointCloudRequest;
using proto::api::component::camera::v1::GetPointCloudResponse;
using proto::api::service::metadata::v1::MetadataService;
using proto::api::service::metadata::v1::ResourcesRequest;
using proto::api::service::metadata::v1::ResourcesResponse;


class MetadataServiceImpl final : public MetadataService::Service  {
 public:
  grpc::Status Resources(ServerContext* context, const ResourcesRequest* request, ResourcesResponse* response) override{
    //define a resource for each component you are including
    ResourceName* name = response->add_resources();    
    name->set_namespace_("rdk");
    name->set_type("component");
    name->set_subtype("camera");
    name->set_name("myCam");
    return grpc::Status::OK;
  }
};

class CameraServiceImpl final : public CameraService::Service {
   public:
    ::grpc::Status GetFrame(ServerContext* context, const GetFrameRequest* request, GetFrameResponse* response) override {
      // Implement functions described by the proto files. 
      // Proto defines GetFrameResponse to have four parts to its message, so we set those here

      // response->set_mime_type("image/both");
      // response->set_width_px(dim_x);
      // response->set_height_px(dim_y);
      // response->set_image(buffer.str()); //an actual implementation has to make buffer.str()
      return grpc::Status::OK;
  }
    ::grpc::Status GetPointCloud(ServerContext* context, const GetPointCloudRequest* request, GetPointCloudResponse* response) override {
      // Camera Service also has a pointcloud response, which has two parts to a message

      // response->set_mime_type("pointcloud/pcd");
      // response->set_point_cloud(buffer.str()); //an actual implementation has to make buffer.str()
      return grpc::Status::OK;
    }
};
int main(const int argc, const char** argv) {
  if (argc < 2) {
    std::cerr << "must supply grpc address" << std::endl;
    return 1;
  }
  MetadataServiceImpl metadataService;
  CameraServiceImpl cameraService;
  ServerBuilder builder;
  builder.AddListeningPort(argv[1], grpc::InsecureServerCredentials());
  builder.RegisterService(&cameraService);
  builder.RegisterService(&metadataService);
  std::unique_ptr<Server> server(builder.BuildAndStart());
  std::cout << "Server listening on " << argv[1] << std::endl;
  server->Wait();
  return 0;
}
