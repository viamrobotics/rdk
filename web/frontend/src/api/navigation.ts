/* eslint-disable no-underscore-dangle */

import * as THREE from 'three';
import { NavigationClient, type Waypoint } from '@viamrobotics/sdk';
import { ViamObject3D } from '@viamrobotics/three';
import type {
  BoxGeometry, CapsuleGeometry, Obstacle, SphereGeometry,
} from './types/navigation';
import { notify } from '@viamrobotics/prime';
export * from './types/navigation';

export const formatWaypoints = (list: Waypoint[]) => {
  return list.map((item) => {
    const { location } = item;
    return {
      id: item.id,
      lng: location?.longitude ?? 0,
      lat: location?.latitude ?? 0,
    };
  });
};

export const getObstacles = async (navClient: NavigationClient): Promise<Obstacle[]> => {
  const list = await navClient.getObstacles();

  return list.map((obstacle, index) => {
    const { location } = obstacle;

    return {
      name: `Obstacle ${index + 1}`,
      location: {
        lng: location?.longitude ?? 0,
        lat: location?.latitude ?? 0,
      },
      geometries: obstacle.geometriesList.map((geometry) => {
        const { center } = geometry;
        const pose = new ViamObject3D();
        const th = THREE.MathUtils.degToRad(center?.theta ?? 0);
        pose.orientationVector.set(center?.oX, center?.oY, center?.oZ, th);

        if (geometry.box) {
          const { dimsMm } = geometry.box;

          return {
            type: 'box',
            length: (dimsMm?.x ?? 0) / 1000,
            width: (dimsMm?.y ?? 0) / 1000,
            height: (dimsMm?.z ?? 0) / 1000,
            pose,
          } satisfies BoxGeometry;

        } else if (geometry.sphere) {

          return {
            type: 'sphere',
            radius: (geometry.sphere.radiusMm ?? 0) / 1000,
            pose,
          } satisfies SphereGeometry;

        } else if (geometry.capsule) {
          const { capsule } = geometry;

          return {
            type: 'capsule',
            radius: (capsule.radiusMm ?? 0) / 1000,
            length: (capsule.lengthMm ?? 0) / 1000,
            pose,
          } satisfies CapsuleGeometry;

        }

        notify.danger('An unsupported geometry was encountered in an obstacle', JSON.stringify(geometry));
        throw new Error(
          `An unsupported geometry was encountered in an obstacle: ${JSON.stringify(geometry)}`
        );
      }),
    } satisfies Obstacle;
  });
};
