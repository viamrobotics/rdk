export type Views = '2D' | '3D'

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
