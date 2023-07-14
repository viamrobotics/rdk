import { createGeometry } from './geometry';

export const createObstacle = (longitude: number, latitude: number) => {
  return {
    location: {
      longitude,
      latitude,
    },
    geometries: [createGeometry('box')],
  };
};
