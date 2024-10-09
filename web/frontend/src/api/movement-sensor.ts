import { rcLogConditionally } from '@/lib/log';
import { type Client, MovementSensorClient } from '@viamrobotics/sdk';

export const getProperties = async (robotClient: Client, name: string) => {
  const client = new MovementSensorClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  return client.getProperties();
};

export const getOrientation = async (robotClient: Client, name: string) => {
  const client = new MovementSensorClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  return client.getOrientation();
};

export const getAngularVelocity = async (robotClient: Client, name: string) => {
  const client = new MovementSensorClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  return client.getAngularVelocity();
};

export const getLinearAcceleration = async (
  robotClient: Client,
  name: string
) => {
  const client = new MovementSensorClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  return client.getLinearAcceleration();
};

export const getLinearVelocity = async (robotClient: Client, name: string) => {
  const client = new MovementSensorClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  return client.getLinearVelocity();
};

export const getCompassHeading = async (robotClient: Client, name: string) => {
  const client = new MovementSensorClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  return client.getCompassHeading();
};

export const getPosition = async (robotClient: Client, name: string) => {
  const client = new MovementSensorClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  return client.getPosition();
};

export const getAccuracy = async (robotClient: Client, name: string) => {
  const client = new MovementSensorClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  return client.getAccuracy();
};
