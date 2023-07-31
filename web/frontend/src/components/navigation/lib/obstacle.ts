import type { LngLat, Obstacle } from '@/api/navigation';
import { createGeometry } from './geometry';
import type { Shapes } from './types';

export const createObstacle = (name: string, lngLat: LngLat, type: Shapes = 'box'): Obstacle => {
  return {
    name,
    location: { ...lngLat },
    geometries: [createGeometry(type)],
  };
};
