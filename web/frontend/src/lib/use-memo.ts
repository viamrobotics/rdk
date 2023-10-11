import { onDestroy } from 'svelte';

const fns = new WeakMap();

/**
 * A hook for calling a callback only once for many hook instances.
 * @param callback
 */
export const useMemo = <T = void>(callback: () => (T | (() => T))): T => {
  const data = fns.get(callback) ?? {
    refCount: 0,
    cleanup: 0,
    returnValue: undefined,
  };

  if (data.refCount === 0) {
    data.returnValue = callback();
  }

  data.refCount += 1;
  fns.set(callback, data);

  onDestroy(() => {
    data.refCount -= 1;

    if (data.refCount === 0) {
      data.cleanup?.();
      fns.delete(callback);
    }
  });

  return data.returnValue;
};
