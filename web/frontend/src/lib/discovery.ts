import { grpc } from '@improbable-eng/grpc-web';
import type { DiscoverComponentsResponse } from '../gen/proto/api/robot/v1/robot_pb';

import type { ServiceError } from '../gen/proto/api/robot/v1/robot_pb_service.esm';

export const fetchCameraDiscoveries = (
  subType: string,
  model: string,
  callback: (error: ServiceError | null, options: DiscoverComponentsResponse | null) => void
) => {
  // Build discovery request
  const req = new window.robotApi.DiscoverComponentsRequest();
  const query = new window.robotApi.DiscoveryQuery();
  query.setSubtype(subType);
  query.setModel(model);
  req.setQueriesList([query]);

  // Make discoveries!
  window.robotService.discoverComponents(req, new grpc.Metadata(), callback);
};
