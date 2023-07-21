import type { navigationApi } from '@viamrobotics/sdk';

export type NavigationModes =
  | typeof navigationApi.Mode.MODE_MANUAL
  | typeof navigationApi.Mode.MODE_UNSPECIFIED
  | typeof navigationApi.Mode.MODE_WAYPOINT

export type LngLat = { lng: number, lat: number }
export type Waypoint = LngLat & { id: string }

export interface Translation {
  x: number;
  y: number;
  z: number;
}

export interface Quaternion {
  x: number;
  y: number;
  z: number;
  w: number;
}

export interface CapsuleGeometry {
  type: 'capsule';
  radius: number;
  length: number;
  quaternion: Quaternion;
}
export interface SphereGeometry {
  type: 'sphere';
  radius: number;
  quaternion: Quaternion;
}

export interface BoxGeometry {
  type: 'box';
  length: number;
  width: number;
  height: number;
  quaternion: Quaternion;
}

export type Geometry = BoxGeometry | SphereGeometry | CapsuleGeometry

export interface Obstacle {
  name: string;
  location: LngLat,
  geometries: Geometry[];
}
