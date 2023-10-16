import { NavigationClient } from '@viamrobotics/sdk';
import { useMemo } from '@/lib/use-memo';
import { useRobotClient } from '@/hooks/robot-client';
import { rcLogConditionally } from '@/lib/log';

export const useNavClient = (name: string) => {
  return useMemo('useNavClient', () => {
    console.log('create client');
    const { robotClient } = useRobotClient();
    return new NavigationClient(robotClient.current, name, { requestLogger: rcLogConditionally });
  });
};
