import { rcLogConditionally } from '@/lib/log';
import {
  EncoderClient,
  EncoderPositionType,
  type Client,
} from '@viamrobotics/sdk';

export const getProperties = async (robotClient: Client, name: string) => {
  const client = new EncoderClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  return client.getProperties();
};

export const getPosition = async (robotClient: Client, name: string) => {
  const client = new EncoderClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  const resp = await client.getPosition();
  return resp[0];
};

export const getPositionDegrees = async (robotClient: Client, name: string) => {
  const client = new EncoderClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  const resp = await client.getPosition(EncoderPositionType.ANGLE_DEGREES);
  return resp[0];
};

export const reset = async (robotClient: Client, name: string) => {
  const client = new EncoderClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  return client.resetPosition();
};
