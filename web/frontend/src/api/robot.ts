import { type Client, robotApi } from '@viamrobotics/sdk';

export const getOperations = async (robotClient: Client) => {
  const request = new robotApi.GetOperationsRequest();

  const response = await new Promise<robotApi.GetOperationsResponse | null>((resolve, reject) => {
    robotClient.robotService.getOperations(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject().operationsList ?? [];
};

export const getResourceNames = async (robotClient: Client) => {
  const request = new robotApi.ResourceNamesRequest();

  const response = await new Promise<robotApi.ResourceNamesResponse | null>((resolve, reject) => {
    robotClient.robotService.resourceNames(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject().resourcesList ?? [];
};

export const getSessions = async (robotClient: Client) => {
  const request = new robotApi.GetSessionsRequest();

  const response = await new Promise<robotApi.GetSessionsResponse | null>((resolve, reject) => {
    robotClient.robotService.getSessions(request, (error, res) => {
      if (error) {
        reject(error);
        return;
      }

      resolve(res);
    });
  });

  return response?.toObject().sessionsList ?? [];
};
