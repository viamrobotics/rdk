import { createGeometry } from './geometry';
import type { Shapes } from './types';

export const createObstacle = (longitude: number, latitude: number, type: Shapes = 'box', name: string) => {
  return {
    name,
    location: {
      longitude,
      latitude,
    },
    geometries: [createGeometry(type)],
  };
};
