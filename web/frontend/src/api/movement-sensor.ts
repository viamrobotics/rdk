import { type Client, movementSensorApi } from '@viamrobotics/sdk';
import type { commonApi } from '@viamrobotics/sdk';
import { rcLogConditionally } from '@/lib/log';

export const getProperties = (client: Client, name: string) => {
  const req = new movementSensorApi.GetPropertiesRequest();
  req.setName(name);

  rcLogConditionally(req);

  return new Promise<movementSensorApi.GetPropertiesResponse.AsObject | undefined>((resolve, reject) => {
    client.movementSensorService.getProperties(req, (error, response) => (
      error ? reject(error) : resolve(response?.toObject())
    ));
  });
};

export const getOrientation = (client: Client, name: string) => {
  const req = new movementSensorApi.GetOrientationRequest();
  req.setName(name);

  rcLogConditionally(req);

  return new Promise<commonApi.Orientation.AsObject | undefined>((resolve, reject) => {
    client.movementSensorService.getOrientation(req, (error, response) => (
      error ? reject(error) : resolve(response?.toObject().orientation)
    ));
  });
};

export const getAngularVelocity = (client: Client, name: string) => {
  const req = new movementSensorApi.GetAngularVelocityRequest();
  req.setName(name);

  rcLogConditionally(req);

  return new Promise<commonApi.Vector3.AsObject | undefined>((resolve, reject) => {
    client.movementSensorService.getAngularVelocity(req, (error, response) => (
      error ? reject(error) : resolve(response?.toObject().angularVelocity)
    ));
  });
};

export const getLinearAcceleration = (client: Client, name: string) => {
  const req = new movementSensorApi.GetLinearAccelerationRequest();
  req.setName(name);

  rcLogConditionally(req);

  return new Promise<commonApi.Vector3.AsObject | undefined>((resolve, reject) => {
    client.movementSensorService.getLinearAcceleration(req, (error, response) => (
      error ? reject(error) : resolve(response?.toObject().linearAcceleration)
    ));
  });
};

export const getLinearVelocity = (client: Client, name: string) => {
  const req = new movementSensorApi.GetLinearVelocityRequest();
  req.setName(name);

  rcLogConditionally(req);

  return new Promise<commonApi.Vector3.AsObject | undefined>((resolve, reject) => {
    client.movementSensorService.getLinearVelocity(req, (error, response) => (
      error ? reject(error) : resolve(response?.toObject().linearVelocity)
    ));
  });
};

export const getCompassHeading = (client: Client, name: string) => {
  const req = new movementSensorApi.GetCompassHeadingRequest();
  req.setName(name);

  rcLogConditionally(req);

  return new Promise<number | undefined>((resolve, reject) => {
    client.movementSensorService.getCompassHeading(req, (error, response) => (
      error ? reject(error) : resolve(response?.toObject().value)
    ));
  });
};

export const getPosition = (client: Client, name: string) => {
  const req = new movementSensorApi.GetPositionRequest();
  req.setName(name);

  rcLogConditionally(req);

  return new Promise<movementSensorApi.GetPositionResponse.AsObject | undefined>((resolve, reject) => {
    client.movementSensorService.getPosition(req, (error, response) => (
      error ? reject(error) : resolve(response?.toObject())
    ));
  });
};
