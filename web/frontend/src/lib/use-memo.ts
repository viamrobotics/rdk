import { onDestroy } from 'svelte';

type Callback<T> = () => (T | (() => T))

interface CallbackData {
  refCount: number;
  returnValue: unknown
}

const fns = new Map<string, CallbackData>();

/**
 * A hook for calling a callback only once for many hook instances.
 * @param callback
 */
export const useMemo = <T = void>(key: string, callback: Callback<T>): T => {
  const data: CallbackData = fns.get(key) ?? {
    refCount: 0,
    returnValue: undefined,
  };

  if (data.refCount === 0) {
    data.returnValue = callback();
  }

  data.refCount += 1;
  fns.set(key, data);

  onDestroy(() => {
    data.refCount -= 1;

    if (data.refCount === 0) {
      fns.delete(key);
    }
  });

  return data.returnValue as T;
};
