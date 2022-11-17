import { grpc } from '@improbable-eng/grpc-web';

import type { robotApi, ServiceError } from '@viamrobotics/sdk';

export const fetchCameraDiscoveries = (
  subType: string,
  model: string,
  callback: (error: ServiceError | null, options: robotApi.DiscoverComponentsResponse | null) => void
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
