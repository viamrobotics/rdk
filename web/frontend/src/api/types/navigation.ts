import type { navigationApi } from '@viamrobotics/sdk';

export type NavigationModes =
  | typeof navigationApi.Mode.MANUAL
  | typeof navigationApi.Mode.UNSPECIFIED
  | typeof navigationApi.Mode.WAYPOINT;

export interface LngLat {
  lng: number;
  lat: number;
}
export type Waypoint = LngLat & { id: string };
