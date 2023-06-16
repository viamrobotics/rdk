import { type Client, robotApi } from '@viamrobotics/sdk';
import { grpc } from '@improbable-eng/grpc-web';

export const fetchCurrentOps = (client: Client) => {
  const req = new robotApi.GetOperationsRequest();

  return new Promise<robotApi.Operation.AsObject[]>((resolve, reject) => {
    client.robotService.getOperations(req, new grpc.Metadata(), (err, response) => {
      if (err) {
        reject(err);
        return;
      }

      if (!response) {
        reject(new Error('An unexpected issue occurred.'));
        return;
      }

      resolve(response.toObject().operationsList ?? []);
    });
  });
};
