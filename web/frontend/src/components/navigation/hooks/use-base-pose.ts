import type { ServiceError } from '@viamrobotics/sdk';
import { useNavClient } from './use-nav-client';
import { writable, get } from 'svelte/store';
import type { LngLat } from '@/api/navigation';
import { setAsyncInterval } from '@/lib/schedule';
import { useDisconnect } from '@/hooks/robot-client';
import { useMemo } from '@/lib/use-memo';

export const useBasePose = (name: string) => {
  return useMemo(() => {
    const navClient = useNavClient(name);
    const pose = writable<LngLat & { rotation: number } | null>(null);
    const error = writable<ServiceError | null>(null);

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

        error.set(null);
        pose.set(position);
      } catch (error_) {
        error.set(error_ as ServiceError);
        pose.set(null);
      }
    };

    updateLocation();
    const clearUpdateLocationInterval = setAsyncInterval(updateLocation, 300);

    useDisconnect(() => clearUpdateLocationInterval());

    return { pose, error };
  });
};
