import { type ServiceError, navigationApi } from '@viamrobotics/sdk';
import { useNavClient } from './use-nav-client';
import { writable } from 'svelte/store';

export type NavigationMode = (
  typeof navigationApi.Mode.MODE_UNSPECIFIED |
  typeof navigationApi.Mode.MODE_MANUAL |
  typeof navigationApi.Mode.MODE_WAYPOINT
)

export const useNavMode = (name: string) => {
  const navClient = useNavClient(name);
  const mode = writable<NavigationMode | undefined>(undefined);
  const error = writable<ServiceError | undefined>(undefined);

  const fetchMode = async () => {
    try {
      mode.set(await navClient.getMode());
      error.set(undefined);
    } catch (error_) {
      mode.set(undefined);
      error.set(error_ as ServiceError);
    }
  };

  const setMode = async (value: NavigationMode) => {
    return navClient.setMode(value);
  };

  fetchMode();

  return { mode, error, setMode };
};
