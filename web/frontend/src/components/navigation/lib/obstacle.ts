import type { Obstacle } from '@/api/navigation';
import { createGeometry } from './geometry';
import type { Shapes } from './types';

export const createObstacle = (name: string, lng: number, lat: number, type: Shapes = 'box'): Obstacle => {
  return {
    name,
    location: { lng, lat },
    geometries: [createGeometry(type)],
  };
};
