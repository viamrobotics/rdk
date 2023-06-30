import { robotApi, type ResourceName } from '@viamrobotics/sdk';
import { grpc } from '@improbable-eng/grpc-web';
import { client } from '@/stores/client';

export const getOperations = () => {
  const request = new robotApi.GetOperationsRequest();

  return new Promise<robotApi.Operation.AsObject[]>((resolve, reject) => {
    client.current.robotService.getOperations(request, new grpc.Metadata(), (error, response) => {
      if (error) {
        reject(error);
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

export const getResourceNames = () => {
  const request = new robotApi.ResourceNamesRequest();

  return new Promise<ResourceName[]>((resolve, reject) => {
    client.current.robotService.resourceNames(request, new grpc.Metadata(), (error, response) => {
      if (error) {
        reject(error);
        return;
      }

      if (!response) {
        reject(new Error('An unexpected issue occured.'));
        return;
      }

      resolve(response.toObject().resourcesList);
    });
  });
};

export const getSessions = () => {
  const request = new robotApi.GetSessionsRequest();

  return new Promise<robotApi.Session.AsObject[]>((resolve, reject) => {
    client.current.robotService.getSessions(request, new grpc.Metadata(), (error, response) => {
      if (error) {
        reject(error);
        return;
      }

      if (!response) {
        reject(new Error('An unexpected issue occurred.'));
        return;
      }

      const list = response.toObject().sessionsList;
      list.sort((sess1, sess2) => (sess1.id < sess2.id ? -1 : 1));
      resolve(list);
    });
  });
};
