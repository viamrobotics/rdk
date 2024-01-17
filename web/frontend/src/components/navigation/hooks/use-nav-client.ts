import { NavigationClient } from '@viamrobotics/sdk';
import { useRobotClient } from '@/hooks/robot-client';
import { rcLogConditionally } from '@/lib/log';

export const useNavClient = (name: string) => {
  const { robotClient } = useRobotClient();
  return new NavigationClient(robotClient.current, name, { requestLogger: rcLogConditionally });
};
