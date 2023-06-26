/* eslint-disable multiline-comment-style */
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
