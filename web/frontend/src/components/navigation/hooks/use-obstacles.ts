import { formatObstacles } from '@/api/navigation';
import { type ServiceError } from '@viamrobotics/sdk';
import type { Obstacle } from '@viamrobotics/prime-blocks';
import { writable } from 'svelte/store';
import { useDisconnect } from '@/hooks/robot-client';
import { setAsyncInterval } from '@/lib/schedule';
import { useNavClient } from './use-nav-client';

export const useObstacles = (name: string) => {
  const navClient = useNavClient(name);
  const obstacles = writable<Obstacle[]>([]);
  const error = writable<ServiceError | undefined>(undefined);

  const updateObstacles = async () => {
    try {
      const response = await navClient.getObstacles();
      error.set(undefined);
      obstacles.set(formatObstacles(response));
    } catch (error_) {
      error.set(error_ as ServiceError);
      obstacles.set([]);
    }
  };

  const clearUpdateObstacleInterval = setAsyncInterval(updateObstacles, 1000);
  void updateObstacles();
  useDisconnect(() => clearUpdateObstacleInterval());

  return { obstacles, error };
};
