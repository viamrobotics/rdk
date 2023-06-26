/* eslint-disable id-length */
import { currentWritable } from '@threlte/core';
import { type FlyToOptions, type Map } from 'maplibre-gl';
import type { Obstacle } from './types';
import type { LngLat, Waypoint } from '@/api/navigation';

type ZoomLevel = Record<string, number>;

export const lngLat = currentWritable<LngLat>({ lng: 0, lat: 0 });
export const robotPosition = currentWritable<LngLat | null>(null);
export const map = currentWritable<Map | null>(null);
export const obstacles = currentWritable<Obstacle[]>([]);

export const zoomLevels = currentWritable<ZoomLevel>({});
export const followRobot = currentWritable(true);

export const waypoints = currentWritable<Waypoint[]>([]);

interface Options {
  center?: boolean
  flyTo?: FlyToOptions
}

export const setLngLat = (value: LngLat, options: Options = {}) => {
  lngLat.set(value);

  if (options.center) {
    map.current?.jumpTo({ center: value });
  }

  if (options.flyTo) {
    map.current?.flyTo({
      zoom: 15,
      duration: 800,
      curve: 0.1,
      ...options.flyTo,
      center: [value.lng, value.lat],
    });
  }
};

/*
 * Mock time!
 * obstacles.set([
 *   {
 *     location: {
 *       latitude: 40.6759,
 *       longitude: -73.958_847,
 *     },
 *     geometries: [
 *       {
 *         type: 'capsule',
 *         r: 10,
 *         l: 30,
 *         translation: { x: 0, y: 0, z: 0 },
 *       },
 *     ],
 *   },
 *   {
 *     location: {
 *       latitude: 40.744_25,
 *       longitude: -73.998_58,
 *     },
 *     geometries: [
 *       {
 *         type: 'capsule',
 *         r: 10,
 *         l: 30,
 *         translation: { x: 0, y: 0, z: 0 },
 *       },
 *     ],
 *   },
 *   {
 *     location: {
 *       latitude: 40.689_95,
 *       longitude: -73.919_41,
 *     },
 *     geometries: [
 *       {
 *         type: 'capsule',
 *         r: 10,
 *         l: 30,
 *         translation: { x: 0, y: 0, z: 0 },
 *       },
 *     ],
 *   },
 *   {
 *     location: {
 *       latitude: 40,
 *       longitude: -74.6,
 *     },
 *     geometries: [
 *       {
 *         type: 'sphere',
 *         r: 1000,
 *         translation: { x: 0, y: 0, z: 0 },
 *       },
 *     ],
 *   },
 *   {
 *     location: {
 *       latitude: 41.461_05,
 *       longitude: -73.913_92,
 *     },
 *     geometries: [
 *       {
 *         type: 'capsule',
 *         r: 10,
 *         l: 30,
 *         translation: { x: 0, y: 0, z: 0 },
 *       },
 *     ],
 *   },
 *   {
 *     location: {
 *       latitude: 40,
 *       longitude: -74.7,
 *     },
 *     geometries: [
 *       {
 *         type: 'box',
 *         x: 100,
 *         y: 100,
 *         z: 100,
 *         translation: { x: 0, y: 0, z: 0 },
 *       },
 *     ],
 *   },
 * ]);
 */
