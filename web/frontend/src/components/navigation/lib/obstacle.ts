import { createGeometry } from './geometry';
import type { Shapes } from './types';

export const createObstacle = (name: string, longitude: number, latitude: number, type: Shapes = 'box') => {
  return {
    name,
    location: {
      longitude,
      latitude,
    },
    geometries: [createGeometry(type)],
  };
};
