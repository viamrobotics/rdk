import { formatPaths } from '@/api/navigation';
import { useConnect } from '@/hooks/robot-client';
import { setAsyncInterval } from '@/lib/schedule';
import { type Path } from '@viamrobotics/prime-blocks';
import { ConnectError } from '@viamrobotics/sdk';
import { writable } from 'svelte/store';
import { useNavClient } from './use-nav-client';

export const usePaths = (name: string) => {
  const navClient = useNavClient(name);
  const paths = writable<Path[]>([]);
  const error = writable<ConnectError | undefined>(undefined);

  const updatePaths = async () => {
    try {
      const response = await navClient.getPaths();
      error.set(undefined);
      paths.set(formatPaths(response));
    } catch (error_) {
      error.set(error_ as ConnectError);
      paths.set([]);
    }
  };

  useConnect(() => {
    void updatePaths();
    const clearInterval = setAsyncInterval(updatePaths, 1000);
    return () => clearInterval();
  });

  return { paths, error };
};
