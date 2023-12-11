/* eslint-disable no-underscore-dangle */

import * as THREE from 'three';
import {
  NavigationClient,
  type Path as SDKPath,
  type Waypoint,
} from '@viamrobotics/sdk';
import { ViamObject3D } from '@viamrobotics/three';
import { notify } from '@viamrobotics/prime';
import { theme } from '@viamrobotics/prime-core/theme';
import type {
  Obstacle,
  BoxGeometry,
  CapsuleGeometry,
  SphereGeometry,
  Path,
} from '@viamrobotics/prime-blocks';
export * from './types/navigation';

const STATIC_OBSTACLE_LABEL = 'static';
const STATIC_OBSTACLE_COLOR = theme.extend.colors.cyberpunk;
const TRANSIENT_OBSTACLE_LABEL = 'transient';
const TRANSIENT_OBSTACLE_COLOR = theme.extend.colors.hologram;

/** Transient obstacles will contain this string in the geometry's label. */
const TRANSIENT_LABEL_SEARCH = 'transientObstacle';

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

export const getObstacles = async (
  navClient: NavigationClient
): Promise<Obstacle[]> => {
  const list = await navClient.getObstacles();

  return list.map((obstacle, index) => {
    const { location } = obstacle;

    /*
     * Labels are defined on each geometry, not on the
     * obstacle itself. In practice, obstacles typically
     * only have a single geometry. This takes the label
     * on the first geometry and uses that even if there
     * are multiple geometries.
     */

    const [geo] = obstacle.geometriesList;

    let name = `Obstacle ${index + 1}`;
    if (obstacle.geometriesList.length > 1) {
      name = `${geo?.label} & others`;
    } else if (geo?.label) {
      name = geo.label;
    }

    const isTransient = geo?.label.includes(TRANSIENT_LABEL_SEARCH);
    const label = isTransient
      ? TRANSIENT_OBSTACLE_LABEL
      : STATIC_OBSTACLE_LABEL;
    const color = isTransient
      ? TRANSIENT_OBSTACLE_COLOR
      : STATIC_OBSTACLE_COLOR;

    return {
      name,
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
            radius: geometry.sphere.radiusMm / 1000,
            pose,
          } satisfies SphereGeometry;
        } else if (geometry.capsule) {
          const { capsule } = geometry;

          return {
            type: 'capsule',
            radius: capsule.radiusMm / 1000,
            length: capsule.lengthMm / 1000,
            pose,
          } satisfies CapsuleGeometry;
        }

        notify.danger(
          'An unsupported geometry was encountered in an obstacle',
          JSON.stringify(geometry)
        );
        throw new Error(
          `An unsupported geometry was encountered in an obstacle: ${JSON.stringify(
            geometry
          )}`
        );
      }),
      label,
      color,
    } satisfies Obstacle;
  });
};

export const formatPaths = (list: SDKPath[]): Path[] => {
  return list.map(({ geopointsList }) =>
    geopointsList.map((geo) => ({
      lng: geo.longitude,
      lat: geo.latitude,
    }))
  );
};
