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

export interface CapsuleGeometry {
  type: 'capsule';
  r: number;
  l: number;
  translation: Translation;
}
export interface SphereGeometry {
  type: 'sphere';
  r: number;
  translation: Translation;
}

export interface BoxGeometry {
  type: 'box';
  x: number;
  y: number;
  z: number;
  translation: Translation;
}

export type Geometry = BoxGeometry | SphereGeometry | CapsuleGeometry

export interface Obstacle {
  name: string;
  location: {
    latitude: number;
    longitude: number;
  },
  geometries: Geometry[];
}
