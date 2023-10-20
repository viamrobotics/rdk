import type { navigationApi } from '@viamrobotics/sdk';
import type { ViamObject3D } from '@viamrobotics/three';

export type NavigationModes =
  | typeof navigationApi.Mode.MODE_MANUAL
  | typeof navigationApi.Mode.MODE_UNSPECIFIED
  | typeof navigationApi.Mode.MODE_WAYPOINT

export interface LngLat { lng: number, lat: number }
export type Waypoint = LngLat & { id: string }

interface BaseGeometry {
  pose: ViamObject3D
}

export type CapsuleGeometry = BaseGeometry & {
  type: 'capsule';
  radius: number;
  length: number;
}

export type SphereGeometry = BaseGeometry & {
  type: 'sphere';
  radius: number;
}

export type BoxGeometry = BaseGeometry & {
  type: 'box';
  length: number;
  width: number;
  height: number;
}

export type Geometry = BoxGeometry | SphereGeometry | CapsuleGeometry

export interface Obstacle {
  name: string;
  location: LngLat,
  geometries: Geometry[];
}
