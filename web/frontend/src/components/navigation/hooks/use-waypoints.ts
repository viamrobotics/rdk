import { type LngLat, formatWaypoints } from '@/api/navigation';
import { type ServiceError } from '@viamrobotics/sdk';
import { type Waypoint } from '@viamrobotics/prime-blocks';
import { writable } from 'svelte/store';
import { useDisconnect } from '@/hooks/robot-client';
import { useMemo } from '@/lib/use-memo';
import { setAsyncInterval } from '@/lib/schedule';
import { useNavClient } from './use-nav-client';

export const useWaypoints = (name: string) => {
  return useMemo('useWaypoint', () => {
    const navClient = useNavClient(name);
    const waypoints = writable<Waypoint[]>([]);
    const error = writable<ServiceError | undefined>(undefined);

    const updateWaypoints = async () => {
      try {
        const response = await navClient.getWayPoints();
        error.set(undefined);
        waypoints.set(formatWaypoints(response));
      } catch (error_) {
        error.set(error_ as ServiceError);
        waypoints.set([]);
      }
    };

    const addWaypoint = async (lngLat: LngLat) => {
      const location = { latitude: lngLat.lat, longitude: lngLat.lng };
      const temp = { lng: lngLat.lng, lat: lngLat.lat, id: crypto.randomUUID() };

      try {
        waypoints.update((value) => {
          value.push(temp);
          return value;
        });
        await navClient.addWayPoint(location);
      } catch (error_) {
        error.set(error_ as ServiceError);
        waypoints.update((value) => value.filter((item) => item.id !== temp.id));
      }
    };

    const deleteWaypoint = async (id: string) => {
      try {
        waypoints.update((value) => value.filter((item) => item.id !== id));
        await navClient.removeWayPoint(id);
      } catch (error_) {
        error.set(error_ as ServiceError);
      }
    };

    const clearUpdateWaypointInterval = setAsyncInterval(updateWaypoints, 1000);
    updateWaypoints();
    useDisconnect(() => clearUpdateWaypointInterval());

    return { waypoints, error, addWaypoint, deleteWaypoint };
  });
};
