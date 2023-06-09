import { type Client, encoderApi } from '@viamrobotics/sdk';
import { rcLogConditionally } from '@/lib/log';
import { grpc } from '@improbable-eng/grpc-web';

export const getProperties = (client: Client, name: string) => {
  const request = new encoderApi.GetPropertiesRequest();
  request.setName(name);

  rcLogConditionally(request);

  return new Promise<encoderApi.GetPropertiesResponse.AsObject | undefined>((resolve, reject) => {
    client.encoderService.getProperties(request, new grpc.Metadata(), (error, response) => (
      error ? reject(error) : resolve(response?.toObject())
    ));
  });
};

export const getPosition = (client: Client, name: string) => {
  const request = new encoderApi.GetPositionRequest();
  request.setName(name);

  rcLogConditionally(request);

  return new Promise<number | undefined>((resolve, reject) => {
    client.encoderService.getPosition(request, new grpc.Metadata(), (error, response) => (
      error ? reject(error) : resolve(response?.toObject().value)
    ));
  });
};

export const getPositionDegrees = (client: Client, name: string) => {
  const request = new encoderApi.GetPositionRequest();
  request.setName(name);
  request.setPositionType(2);

  rcLogConditionally(request);

  return new Promise<number | undefined>((resolve, reject) => {
    client.encoderService.getPosition(request, new grpc.Metadata(), (error, response) => (
      error ? reject(error) : resolve(response?.toObject().value)
    ));
  });
};

export const reset = (client: Client, name: string) => {
  const request = new encoderApi.ResetPositionRequest();
  request.setName(name);

  rcLogConditionally(request);

  return new Promise((resolve, reject) => {
    client.encoderService.resetPosition(request, new grpc.Metadata(), (error) => (
      error ? reject(error) : resolve(null)
    ));
  });
};
