/* eslint-disable no-underscore-dangle */

import * as THREE from 'three';
import { type Client, commonApi, navigationApi } from '@viamrobotics/sdk';
import { ViamObject3D } from '@viamrobotics/three';
import { rcLogConditionally } from '@/lib/log';
import type {
  BoxGeometry, CapsuleGeometry, Obstacle, SphereGeometry, Waypoint,
} from './types/navigation';
import { notify } from '@viamrobotics/prime';
export * from './types/navigation';

const formatWaypoints = (list: navigationApi.Waypoint[]) => {
  return list.map((item) => {
    const location = item.getLocation();
    return {
      id: item.getId(),
      lng: location?.getLongitude() ?? 0,
      lat: location?.getLatitude() ?? 0,
    };
  });
};

export const getObstacles = async (robotClient: Client, name: string): Promise<Obstacle[]> => {
  const req = new navigationApi.GetObstaclesRequest();
  req.setName(name);

  rcLogConditionally(req);

  const response = await new Promise<navigationApi.GetObstaclesResponse | null>((resolve, reject) => {
    robotClient.navigationService.getObstacles(req, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  const list = response?.getObstaclesList() ?? [];

  return list.map((obstacle, index) => {
    const location = obstacle.getLocation();

    return {
      name: `Obstacle ${index + 1}`,
      location: {
        lng: location?.getLongitude() ?? 0,
        lat: location?.getLatitude() ?? 0,
      },
      geometries: obstacle.getGeometriesList().map((geometry) => {
        const center = geometry.getCenter();
        const pose = new ViamObject3D();
        const th = THREE.MathUtils.degToRad(center?.getTheta() ?? 0);
        pose.orientationVector.set(center?.getOX(), center?.getOY(), center?.getOZ(), th);

        if (geometry.hasBox()) {
          const dimsMm = geometry.getBox()?.getDimsMm();

          return {
            type: 'box',
            length: (dimsMm?.getX() ?? 0) / 1000,
            width: (dimsMm?.getY() ?? 0) / 1000,
            height: (dimsMm?.getZ() ?? 0) / 1000,
            pose,
          } satisfies BoxGeometry;

        } else if (geometry.hasSphere()) {

          return {
            type: 'sphere',
            radius: (geometry.getSphere()?.getRadiusMm() ?? 0) / 1000,
            pose,
          } satisfies SphereGeometry;

        } else if (geometry.hasCapsule()) {
          const capsule = geometry.getCapsule();

          return {
            type: 'capsule',
            radius: (capsule?.getRadiusMm() ?? 0) / 1000,
            length: (capsule?.getLengthMm() ?? 0) / 1000,
            pose,
          } satisfies CapsuleGeometry;

        }

        notify.danger('An unsupported geometry was encountered in an obstacle', JSON.stringify(geometry.toObject()));
        throw new Error(
          `An unsupported geometry was encountered in an obstacle: ${JSON.stringify(geometry.toObject())}`
        );
      }),
    } satisfies Obstacle;
  });
};

export const getWaypoints = async (robotClient: Client, name: string): Promise<Waypoint[]> => {
  const req = new navigationApi.GetWaypointsRequest();
  req.setName(name);

  rcLogConditionally(req);

  const response = await new Promise<{ getWaypointsList(): navigationApi.Waypoint[] } | null>((resolve, reject) => {
    robotClient.navigationService.getWaypoints(req, (error, res) => {
      if (error) {
        reject(error);
      } else {
        resolve(res);
      }
    });
  });

  return formatWaypoints(response?.getWaypointsList() ?? []);
};
