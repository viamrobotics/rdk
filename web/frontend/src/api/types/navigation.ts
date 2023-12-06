import type { navigationApi } from '@viamrobotics/sdk';

export type NavigationModes =
  | typeof navigationApi.Mode.MODE_MANUAL
  | typeof navigationApi.Mode.MODE_UNSPECIFIED
  | typeof navigationApi.Mode.MODE_WAYPOINT;

export interface LngLat {
  lng: number;
  lat: number;
}
export type Waypoint = LngLat & { id: string };
