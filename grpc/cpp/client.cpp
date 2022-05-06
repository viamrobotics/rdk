#include <iostream>

#include <grpc/grpc.h>
#include <grpcpp/channel.h>
#include <grpcpp/client_context.h>
#include <grpcpp/create_channel.h>
#include <google/protobuf/util/json_util.h>
#include <grpcpp/security/credentials.h>

#include "proto/api/service/metadata/v1/metadata.grpc.pb.h"
#include "proto/api/service/metadata/v1/metadata.pb.h"
#include "proto/api/common/v1/common.grpc.pb.h"
#include "proto/api/common/v1/common.pb.h"

using grpc::Channel;
using grpc::ClientContext;
using grpc::Status;
using proto::api::service::metadata::v1::MetadataService;
using proto::api::service::metadata::v1::ResourcesRequest;
using proto::api::service::metadata::v1::ResourcesResponse;
using proto::api::common::v1::ResourceName;

int main(const int argc, const char** argv) {
  if (argc < 2) {
    std::cerr << "must supply grpc address" << std::endl;
    return 1;
  }
  const std::shared_ptr<Channel> channel = grpc::CreateChannel(argv[1], grpc::InsecureChannelCredentials());
  const std::unique_ptr<MetadataService::Stub> client = MetadataService::NewStub(channel);
  ClientContext context;
  const ResourcesRequest request;
  ResourcesResponse response;
  const Status gStatus = client->Resources(&context, request, &response);
  if (!gStatus.ok()) {
    std::cout << "Status rpc failed." << std::endl;
    return 1;
  }
  std::string json;
  google::protobuf::util::MessageToJsonString(response, &json);
  std::cout << json << std::endl;
  return 0;
}
