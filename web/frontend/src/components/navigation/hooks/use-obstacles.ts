import { formatObstacles } from '@/api/navigation';
import { useConnect } from '@/hooks/robot-client';
import { setAsyncInterval } from '@/lib/schedule';
import type { Obstacle } from '@viamrobotics/prime-blocks';
import { ConnectError } from '@viamrobotics/sdk';
import { writable } from 'svelte/store';
import { useNavClient } from './use-nav-client';

export const useObstacles = (name: string) => {
  const navClient = useNavClient(name);
  const obstacles = writable<Obstacle[]>([]);
  const error = writable<ConnectError | undefined>(undefined);

  const updateObstacles = async () => {
    try {
      const response = await navClient.getObstacles();
      error.set(undefined);
      obstacles.set(formatObstacles(response));
    } catch (error_) {
      error.set(error_ as ConnectError);
      obstacles.set([]);
    }
  };

  useConnect(() => {
    void updateObstacles();
    const clearInterval = setAsyncInterval(updateObstacles, 1000);
    return () => clearInterval();
  });

  return { obstacles, error };
};
