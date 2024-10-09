import { type LngLat, formatWaypoints } from '@/api/navigation';
import { useConnect } from '@/hooks/robot-client';
import { setAsyncInterval } from '@/lib/schedule';
import { Waypoint } from '@viamrobotics/prime-blocks';
import { ConnectError } from '@viamrobotics/sdk';
import { writable } from 'svelte/store';
import { useNavClient } from './use-nav-client';

export const useWaypoints = (name: string) => {
  const navClient = useNavClient(name);
  const waypoints = writable<Waypoint[]>([]);
  const error = writable<ConnectError | undefined>(undefined);

  const updateWaypoints = async () => {
    try {
      const response = await navClient.getWayPoints();
      error.set(undefined);
      waypoints.set(formatWaypoints(response));
    } catch (error_) {
      error.set(error_ as ConnectError);
      waypoints.set([]);
    }
  };

  const addWaypoint = async (lngLat: LngLat) => {
    const location = { latitude: lngLat.lat, longitude: lngLat.lng };
    const temp = new Waypoint(lngLat.lng, lngLat.lat, crypto.randomUUID());

    try {
      waypoints.update((value) => {
        value.push(temp);
        return value;
      });
      await navClient.addWayPoint(location);
    } catch (error_) {
      error.set(error_ as ConnectError);
      waypoints.update((value) => value.filter((item) => item.id !== temp.id));
    }
  };

  const deleteWaypoint = async (id: string) => {
    try {
      waypoints.update((value) => value.filter((item) => item.id !== id));
      await navClient.removeWayPoint(id);
    } catch (error_) {
      error.set(error_ as ConnectError);
    }
  };

  useConnect(() => {
    void updateWaypoints();
    const clearUpdateWaypointInterval = setAsyncInterval(updateWaypoints, 1000);
    return () => clearUpdateWaypointInterval();
  });

  return { waypoints, error, addWaypoint, deleteWaypoint };
};
