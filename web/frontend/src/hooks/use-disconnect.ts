import { statusStream } from '@/stores/streams';
import { onDestroy } from 'svelte';

export const useDisconnect = (callback: () => void) => {
  statusStream.subscribe((update) => update?.on('end', callback));

  onDestroy(callback);
};
