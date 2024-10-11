import { useConnect } from '@/hooks/robot-client';
import { setAsyncInterval } from '@/lib/schedule';
import { GeoPose } from '@viamrobotics/prime-blocks';
import { ConnectError } from '@viamrobotics/sdk';
import { get, writable } from 'svelte/store';
import { useNavClient } from './use-nav-client';

export const useBasePose = (name: string) => {
  const navClient = useNavClient(name);
  const pose = writable<GeoPose | undefined>(undefined);
  const error = writable<ConnectError | undefined>(undefined);

  const updateLocation = async () => {
    try {
      const { location, compassHeading } = await navClient.getLocation();

      if (!location) {
        return;
      }

      const { latitude, longitude } = location;

      const position = new GeoPose(longitude, latitude, compassHeading);
      const { lat, lng, rotation } = get(pose) ?? {};

      if (
        lat === position.lat &&
        lng === position.lng &&
        rotation === position.rotation
      ) {
        return;
      }

      error.set(undefined);
      pose.set(position);
    } catch (error_) {
      error.set(error_ as ConnectError);
      pose.set(undefined);
    }
  };

  useConnect(() => {
    updateLocation();
    const clearInterval = setAsyncInterval(updateLocation, 300);
    return () => clearInterval();
  });

  return { pose, error };
};
