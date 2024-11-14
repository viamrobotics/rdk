import { rcLogConditionally } from '@/lib/log';
import { ServoClient, type Client } from '@viamrobotics/sdk';

export const move = async (
  robotClient: Client,
  name: string,
  angle: number
) => {
  const client = new ServoClient(robotClient, name, {
    requestLogger: rcLogConditionally,
  });
  return client.move(angle);
};
