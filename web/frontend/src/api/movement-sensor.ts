import { type Client, movementSensorApi } from '@viamrobotics/sdk';
import { rcLogConditionally } from '@/lib/log';

export const getProperties = async (client: Client, name: string) => {
  const req = new movementSensorApi.GetPropertiesRequest();
  req.setName(name);

  rcLogConditionally(req);

  const response = await new Promise<movementSensorApi.GetPropertiesResponse | null>((resolve, reject) => {
    client.movementSensorService.getProperties(req, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject();
};

export const getOrientation = async (client: Client, name: string) => {
  const req = new movementSensorApi.GetOrientationRequest();
  req.setName(name);

  rcLogConditionally(req);

  const response = await new Promise<movementSensorApi.GetOrientationResponse | null>((resolve, reject) => {
    client.movementSensorService.getOrientation(req, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject().orientation;
};

export const getAngularVelocity = async (client: Client, name: string) => {
  const req = new movementSensorApi.GetAngularVelocityRequest();
  req.setName(name);

  rcLogConditionally(req);

  const response = await new Promise<movementSensorApi.GetAngularVelocityResponse | null>((resolve, reject) => {
    client.movementSensorService.getAngularVelocity(req, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject().angularVelocity;
};

export const getLinearAcceleration = async (client: Client, name: string) => {
  const req = new movementSensorApi.GetLinearAccelerationRequest();
  req.setName(name);

  rcLogConditionally(req);

  const response = await new Promise<movementSensorApi.GetLinearAccelerationResponse | null>((resolve, reject) => {
    client.movementSensorService.getLinearAcceleration(req, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject().linearAcceleration;
};

export const getLinearVelocity = async (client: Client, name: string) => {
  const req = new movementSensorApi.GetLinearVelocityRequest();
  req.setName(name);

  rcLogConditionally(req);

  const response = await new Promise<movementSensorApi.GetLinearVelocityResponse | null>((resolve, reject) => {
    client.movementSensorService.getLinearVelocity(req, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject().linearVelocity;
};

export const getCompassHeading = async (client: Client, name: string) => {
  const req = new movementSensorApi.GetCompassHeadingRequest();
  req.setName(name);

  rcLogConditionally(req);

  const response = await new Promise<movementSensorApi.GetCompassHeadingResponse | null>((resolve, reject) => {
    client.movementSensorService.getCompassHeading(req, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject().value;
};

export const getPosition = async (client: Client, name: string) => {
  const req = new movementSensorApi.GetPositionRequest();
  req.setName(name);

  rcLogConditionally(req);

  const response = await new Promise<movementSensorApi.GetPositionResponse | null>((resolve, reject) => {
    client.movementSensorService.getPosition(req, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject();
};
