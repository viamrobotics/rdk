import { ConnectError, navigationApi } from '@viamrobotics/sdk';
import { writable } from 'svelte/store';
import { useNavClient } from './use-nav-client';

export type NavigationMode = navigationApi.Mode;

export const useNavMode = (name: string) => {
  const navClient = useNavClient(name);
  const mode = writable<NavigationMode | undefined>(undefined);
  const error = writable<ConnectError | undefined>(undefined);

  const fetchMode = async () => {
    try {
      mode.set(await navClient.getMode());
      error.set(undefined);
    } catch (error_) {
      mode.set(undefined);
      error.set(error_ as ConnectError);
    }
  };

  const setMode = async (value: NavigationMode) => {
    try {
      await navClient.setMode(value);
      mode.set(value);
      error.set(undefined);
    } catch (error_) {
      error.set(error_ as ConnectError);
    }
  };

  fetchMode();

  return { mode, error, setMode };
};
