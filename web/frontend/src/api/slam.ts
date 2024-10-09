import { SlamClient, type Client } from '@viamrobotics/sdk';

export const getPosition = async (robotClient: Client, name: string) => {
  const client = new SlamClient(robotClient, name);
  const pos = await client.getPosition();
  return pos.pose;
};
