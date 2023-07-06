import type { Client } from '@viamrobotics/sdk';
import { commonApi, navigationApi } from '@viamrobotics/sdk';
import { rcLogConditionally } from '@/lib/log';

export type NavigationModes =
  | typeof navigationApi.Mode.MODE_MANUAL
  | typeof navigationApi.Mode.MODE_UNSPECIFIED
  | typeof navigationApi.Mode.MODE_WAYPOINT

export type LngLat = { lng: number, lat: number }
export type Waypoint = LngLat & { id: string }

export const setMode = async (robotClient: Client, name: string, mode: NavigationModes) => {
  const request = new navigationApi.SetModeRequest();
  request.setName(name);
  request.setMode(mode);

  rcLogConditionally(request);

  const response = await new Promise<navigationApi.SetModeResponse | null>((resolve, reject) => {
    robotClient.navigationService.setMode(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject();
};

export const setWaypoint = async (robotClient: Client, lat: number, lng: number, name: string) => {
  const request = new navigationApi.AddWaypointRequest();
  const point = new commonApi.GeoPoint();

  point.setLatitude(lat);
  point.setLongitude(lng);
  request.setName(name);
  request.setLocation(point);

  rcLogConditionally(request);

  const response = await new Promise<navigationApi.AddWaypointResponse | null>((resolve, reject) => {
    robotClient.navigationService.addWaypoint(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject();
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

export const getWaypoints = async (robotClient: Client, name: string): Promise<Waypoint[]> => {
  const req = new navigationApi.GetWaypointsRequest();
  req.setName(name);

  rcLogConditionally(req);

  const response = await new Promise<{ getWaypointsList(): navigationApi.Waypoint[] } | null>((resolve, reject) => {
    robotClient.navigationService.getWaypoints(req, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return formatWaypoints(response?.getWaypointsList() ?? []);
};

export const removeWaypoint = async (robotClient: Client, name: string, id: string) => {
  const request = new navigationApi.RemoveWaypointRequest();
  request.setName(name);
  request.setId(id);

  rcLogConditionally(request);

  const response = await new Promise<navigationApi.RemoveWaypointResponse | null>((resolve, reject) => {
    robotClient.navigationService.removeWaypoint(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return response?.toObject();
};

export const getLocation = async (robotClient: Client, name: string) => {
  const request = new navigationApi.GetLocationRequest();
  request.setName(name);

  rcLogConditionally(request);

  const response = await new Promise<navigationApi.GetLocationResponse | null>((resolve, reject) => {
    robotClient.navigationService.getLocation(request, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return {
    lat: response?.getLocation()?.getLatitude() ?? 0,
    lng: response?.getLocation()?.getLongitude() ?? 0,
  };
};
