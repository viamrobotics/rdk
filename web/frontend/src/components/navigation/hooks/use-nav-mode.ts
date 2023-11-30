import { type ServiceError, navigationApi } from '@viamrobotics/sdk';
import { useNavClient } from './use-nav-client';
import { writable } from 'svelte/store';

export type NavigationMode = navigationApi.ModeMap[keyof navigationApi.ModeMap];

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
    try {
      await navClient.setMode(value);
      mode.set(value);
      error.set(undefined);
    } catch (error_) {
      error.set(error_ as ServiceError);
    }
  };

  fetchMode();

  return { mode, error, setMode };
};
