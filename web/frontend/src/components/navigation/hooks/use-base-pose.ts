import type { ServiceError } from '@viamrobotics/sdk';
import type { GeoPose } from '@viamrobotics/prime-blocks';
import { useNavClient } from './use-nav-client';
import { writable, get } from 'svelte/store';
import { setAsyncInterval } from '@/lib/schedule';
import { useDisconnect } from '@/hooks/robot-client';
import { useMemo } from '@/lib/use-memo';

export const useBasePose = (name: string) => {
  return useMemo('useBasePose', () => {
    const navClient = useNavClient(name);
    const pose = writable<GeoPose | undefined>(undefined);
    const error = writable<ServiceError | undefined>(undefined);

    const updateLocation = async () => {
      try {
        const { location, compassHeading } = await navClient.getLocation();

        if (!location) {
          return;
        }

        const { latitude, longitude } = location;

        const position = { lat: latitude, lng: longitude, rotation: compassHeading };
        const { lat, lng, rotation } = get(pose) ?? {};

        if (lat === position.lat && lng === position.lng && rotation === position.rotation) {
          return;
        }

        error.set(undefined);
        pose.set(position);
      } catch (error_) {
        error.set(error_ as ServiceError);
        pose.set(undefined);
      }
    };

    updateLocation();
    const clearUpdateLocationInterval = setAsyncInterval(updateLocation, 300);

    useDisconnect(() => clearUpdateLocationInterval());

    return { pose, error };
  });
};
