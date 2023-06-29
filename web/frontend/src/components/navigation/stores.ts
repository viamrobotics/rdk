/* eslint-disable multiline-comment-style */
/* eslint-disable id-length */
import { currentWritable } from '@threlte/core';
import { JumpToOptions, type FlyToOptions, type Map } from 'maplibre-gl';
import type { Obstacle } from './types';
import type { LngLat, Waypoint } from '@/api/navigation';

type ZoomLevel = Record<string, number>;

export const mapCenter = currentWritable<LngLat>({ lng: 0, lat: 0 });
export const robotPosition = currentWritable<LngLat | null>(null);
export const map = currentWritable<Map | null>(null);
export const obstacles = currentWritable<Obstacle[]>([]);
export const zoomLevels = currentWritable<ZoomLevel>({});
export const waypoints = currentWritable<Waypoint[]>([]);

export const flyToMap = (value: LngLat, options: FlyToOptions = {}) => {
  mapCenter.set(value);

  map.current?.flyTo({
    zoom: 15,
    duration: 800,
    curve: 0.1,
    ...options,
    center: [value.lng, value.lat],
  });
};

export const centerMap = (value: LngLat, jumpTo: JumpToOptions | boolean = true) => {
  mapCenter.set(value);

  if (jumpTo) {
    map.current?.jumpTo({
      center: value,
      ...(jumpTo === true ? {} : jumpTo),
    });
  }
};
