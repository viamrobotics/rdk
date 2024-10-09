import { rcLogConditionally } from '@/lib/log';
import { type Client, PowerSensorClient } from '@viamrobotics/sdk';

export const getVoltage = async (robotClient: Client, name: string) => {
  const client = new PowerSensorClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  const resp = await client.getVoltage();
  return resp[0];
};

export const getCurrent = async (robotClient: Client, name: string) => {
  const client = new PowerSensorClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  const resp = await client.getCurrent();
  return resp[0];
};

export const getPower = async (robotClient: Client, name: string) => {
  const client = new PowerSensorClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  return client.getPower();
};
