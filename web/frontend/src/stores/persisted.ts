import { writable, type Writable } from 'svelte/store';

declare type Updater<T> = (value: T) => T;
declare type StoreDict<T> = { [key: string]: Writable<T> }

const stores: StoreDict<unknown> = {};

type StorageType = 'local' | 'session'

const getStorage = (type: StorageType) => {
  return type === 'local' ? localStorage : sessionStorage;
};

/**
 * Creates a writable store that persists in localStorage
 * @param key The storage key
 * @param initialValue The initial value to put in storage if no value exists.
 * @param storageType 'local' or 'session'
 */
export const persisted = <T>(
  key: string,
  initialValue: T,
  storageType: 'local' | 'session' = 'local'
): Writable<T> => {
  const browser = typeof window !== 'undefined' && typeof document !== 'undefined';
  const storage = browser ? getStorage(storageType) : null;

  const updateStorage = (storagekey: string, value: T) => {
    storage?.setItem(storagekey, JSON.stringify(value));
  };

  if (!stores[key]) {
    const store = writable(initialValue, (set) => {
      const json = storage?.getItem(key);

      if (json) {
        set(<T>JSON.parse(json));
      } else {
        updateStorage(key, initialValue);
      }

      if (browser) {
        const handleStorage = (event: StorageEvent) => {
          if (event.key === key) {
            set(event.newValue ? JSON.parse(event.newValue) : null);
          }
        };

        window.addEventListener('storage', handleStorage);

        return () => window.removeEventListener('storage', handleStorage);
      }

      return () => null;
    });

    const { subscribe, set } = store;

    stores[key] = {
      set (value: T) {
        updateStorage(key, value);
        set(value);
      },
      update (callback: Updater<T>) {
        return store.update((last) => {
          const value = callback(last);

          updateStorage(key, value);

          return value;
        });
      },
      subscribe,
    };
  }

  return stores[key] as Writable<T>;
};
