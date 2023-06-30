import { onDestroy } from 'svelte';
import { useClient } from './use-client';

export const useDisconnect = (callback: () => void) => {
  const { statusStream } = useClient();

  statusStream.subscribe((update) => update?.on('end', callback));

  onDestroy(callback);
};
