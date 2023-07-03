import { type Client, encoderApi } from '@viamrobotics/sdk';
import { rcLogConditionally } from '@/lib/log';

export const getProperties = async (client: Client, name: string) => {
  const request = new encoderApi.GetPropertiesRequest();
  request.setName(name);

  rcLogConditionally(request);

  const response = await new Promise<encoderApi.GetPropertiesResponse | null>((resolve, reject) => {
    client.encoderService.getProperties(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject();
};

export const getPosition = async (client: Client, name: string) => {
  const request = new encoderApi.GetPositionRequest();
  request.setName(name);

  rcLogConditionally(request);

  const response = await new Promise<encoderApi.GetPositionResponse | null>((resolve, reject) => {
    client.encoderService.getPosition(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject().value;
};

export const getPositionDegrees = async (client: Client, name: string) => {
  const request = new encoderApi.GetPositionRequest();
  request.setName(name);
  request.setPositionType(2);

  rcLogConditionally(request);

  const response = await new Promise<encoderApi.GetPositionResponse | null>((resolve, reject) => {
    client.encoderService.getPosition(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject().value;
};

export const reset = async (client: Client, name: string) => {
  const request = new encoderApi.ResetPositionRequest();
  request.setName(name);

  rcLogConditionally(request);

  const response = await new Promise<encoderApi.ResetPositionResponse | null>((resolve, reject) => {
    client.encoderService.resetPosition(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject();
};
