import type { Client } from '@viamrobotics/sdk';
import { commonApi, navigationApi } from '@viamrobotics/sdk';
import { rcLogConditionally } from '@/lib/log';

export type NavigationModes =
  | typeof navigationApi.Mode.MODE_MANUAL
  | typeof navigationApi.Mode.MODE_UNSPECIFIED
  | typeof navigationApi.Mode.MODE_WAYPOINT

export type LngLat = { lng: number, lat: number }
export type Waypoint = LngLat & { id: string }

export const setMode = (client: Client, name: string, mode: NavigationModes) => {
  const request = new navigationApi.SetModeRequest();
  request.setName(name);
  request.setMode(mode);

  rcLogConditionally(request);

  return new Promise((resolve, reject) => {
    client.navigationService.setMode(request, (error) => {
      if (error) {
        reject(error);
      } else {
        resolve(null);
      }
    });
  });
};

export const setWaypoint = (client: Client, lat: number, lng: number, name: string) => {
  const request = new navigationApi.AddWaypointRequest();
  const point = new commonApi.GeoPoint();

  point.setLatitude(lat);
  point.setLongitude(lng);
  request.setName(name);
  request.setLocation(point);

  rcLogConditionally(request);

  return new Promise((resolve, reject) => {
    client.navigationService.addWaypoint(request, (error, response) => {
      if (error) {
        reject(error);
      } else {
        resolve(response);
      }
    });
  });
};

const formatWaypoints = (list: navigationApi.Waypoint[]) => {
  return list.map((item) => {
    const location = item.getLocation();
    return {
      id: item.getId(),
      lng: location?.getLongitude() ?? 0,
      lat: location?.getLatitude() ?? 0,
    };
  });
};

export const getWaypoints = async (client: Client, name: string): Promise<Waypoint[]> => {
  const req = new navigationApi.GetWaypointsRequest();
  req.setName(name);

  rcLogConditionally(req);

  const response = await new Promise<{ getWaypointsList(): navigationApi.Waypoint[] } | null>((resolve, reject) => {
    client.navigationService.getWaypoints(req, (error, resp) => {
      if (error) {
        reject(error);
      } else {
        resolve(resp);
      }
    });
  });

  return formatWaypoints(response?.getWaypointsList() ?? []);
};

export const removeWaypoint = (client: Client, name: string, id: string) => {
  const request = new navigationApi.RemoveWaypointRequest();
  request.setName(name);
  request.setId(id);

  rcLogConditionally(request);

  return new Promise((resolve, reject) => {
    client.navigationService.removeWaypoint(request, (error) => {
      if (error) {
        reject(error);
      } else {
        resolve(null);
      }
    });
  });
};

export const getLocation = (client: Client, name: string) => {
  const request = new navigationApi.GetLocationRequest();
  request.setName(name);

  rcLogConditionally(request);

  return new Promise<{ lat: number, lng: number }>((resolve, reject) => {
    client.navigationService.getLocation(request, (error, response) => {
      if (error) {
        reject(error);
      } else {
        resolve({
          lat: response?.getLocation()?.getLatitude() ?? 0,
          lng: response?.getLocation()?.getLongitude() ?? 0,
        });
      }
    });
  });
};
