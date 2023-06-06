import { Client, commonApi, robotApi, navigationApi, type ServiceError, type ResponseStream } from '@viamrobotics/sdk';
import { grpc } from '@improbable-eng/grpc-web';
import { rcLogConditionally } from '@/lib/log';

export const setMode = (
  client: Client,
  name: string,
  mode:
  | typeof navigationApi.Mode.MODE_MANUAL
  | typeof navigationApi.Mode.MODE_UNSPECIFIED
  | typeof navigationApi.Mode.MODE_WAYPOINT
) => {
  return new Promise((resolve, reject) => {
    const req = new navigationApi.SetModeRequest();
    req.setName(name);
    req.setMode(mode);

    rcLogConditionally(req);
    client.navigationService.setMode(req, new grpc.Metadata(), (error: ServiceError | null) => (
      error ? reject(error) : resolve(null)
    ));
  });
};
