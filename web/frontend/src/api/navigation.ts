/* eslint-disable no-underscore-dangle */

import { notify } from '@viamrobotics/prime';
import {
  type BoxGeometry,
  type CapsuleGeometry,
  type Obstacle,
  type Path,
  type SphereGeometry,
  Waypoint,
} from '@viamrobotics/prime-blocks';
import { theme } from '@viamrobotics/prime-core/theme';
import type {
  GeoGeometry,
  Path as SDKPath,
  Waypoint as SDKWaypoint,
} from '@viamrobotics/sdk';
import { ViamObject3D } from '@viamrobotics/three';
import { LngLat } from 'maplibre-gl';
import * as THREE from 'three';
export * from './types/navigation';

const STATIC_OBSTACLE_LABEL = 'static';
const STATIC_OBSTACLE_COLOR = theme.extend.colors.cyberpunk;
const TRANSIENT_OBSTACLE_LABEL = 'transient';
const TRANSIENT_OBSTACLE_COLOR = theme.extend.colors.hologram;

/** Transient obstacles will start with this string in the geometry's label. */
const TRANSIENT_LABEL_SEARCH = 'transient';

export const formatWaypoints = (list: SDKWaypoint[]) => {
  return list.map((item) => {
    const { location } = item;
    return new Waypoint(
      location?.longitude ?? 0,
      location?.latitude ?? 0,
      item.id
    );
  });
};

export const formatObstacles = (list: GeoGeometry[]): Obstacle[] => {
  return list.map((obstacle, index) => {
    const { location } = obstacle;

    /*
     * Labels are defined on each geometry, not on the
     * obstacle itself. In practice, obstacles typically
     * only have a single geometry. This takes the label
     * on the first geometry and uses that even if there
     * are multiple geometries.
     */

    const [geo] = obstacle.geometries;

    let name = `Obstacle ${index + 1}`;
    if (obstacle.geometries.length > 1) {
      name = `${geo?.label} & others`;
    } else if (geo?.label) {
      name = geo.label;
    }

    const isTransient = geo?.label.startsWith(TRANSIENT_LABEL_SEARCH);
    const label = isTransient
      ? TRANSIENT_OBSTACLE_LABEL
      : STATIC_OBSTACLE_LABEL;
    const color = isTransient
      ? TRANSIENT_OBSTACLE_COLOR
      : STATIC_OBSTACLE_COLOR;

    return {
      name,
      location: new LngLat(location?.longitude ?? 0, location?.latitude ?? 0),
      geometries: obstacle.geometries.map((geometry) => {
        const { center } = geometry;
        const pose = new ViamObject3D();
        const th = THREE.MathUtils.degToRad(center?.theta ?? 0);
        pose.orientationVector.set(center?.oX, center?.oY, center?.oZ, th);

        switch (geometry.geometryType.case) {
          case 'box': {
            const { dimsMm } = geometry.geometryType.value;

            return {
              type: 'box',
              length: (dimsMm?.x ?? 0) / 1000,
              width: (dimsMm?.y ?? 0) / 1000,
              height: (dimsMm?.z ?? 0) / 1000,
              pose,
            } satisfies BoxGeometry;
          }
          case 'sphere': {
            return {
              type: 'sphere',
              radius: geometry.geometryType.value.radiusMm / 1000,
              pose,
            } satisfies SphereGeometry;
          }
          case 'capsule': {
            return {
              type: 'capsule',
              radius: geometry.geometryType.value.radiusMm / 1000,
              length: geometry.geometryType.value.lengthMm / 1000,
              pose,
            } satisfies CapsuleGeometry;
          }
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
  return list.map(({ geopoints }) =>
    geopoints.map((geo) => new LngLat(geo.longitude, geo.latitude))
  );
};
