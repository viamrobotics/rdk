import { formatPaths } from '@/api/navigation';
import { type ServiceError } from '@viamrobotics/sdk';
import { type Path } from '@viamrobotics/prime-blocks';
import { writable } from 'svelte/store';
import { useDisconnect } from '@/hooks/robot-client';
import { setAsyncInterval } from '@/lib/schedule';
import { useNavClient } from './use-nav-client';

export const usePaths = (name: string) => {
  const navClient = useNavClient(name);
  const paths = writable<Path[]>([]);
  const error = writable<ServiceError | undefined>(undefined);

  const updatePaths = async () => {
    try {
      const response = await navClient.getPaths();
      error.set(undefined);
      paths.set(formatPaths(response));
    } catch (error_) {
      error.set(error_ as ServiceError);
      paths.set([]);
    }
  };

  const clearUpdatePathsInterval = setAsyncInterval(updatePaths, 1000);
  void updatePaths();
  useDisconnect(() => clearUpdatePathsInterval());

  return { paths, error };
};
