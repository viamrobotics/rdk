import * as THREE from 'three';
import { currentWritable } from '@threlte/core';
import { type JumpToOptions, type FlyToOptions, type Map } from 'maplibre-gl';
import type { LngLat, Waypoint, Obstacle } from '@/api/navigation';
import { persisted } from '@/stores/persisted';

export const mapCenter = currentWritable<LngLat>({ lng: 0, lat: 0 });
export const mapZoom = currentWritable(0);
export const mapSize = currentWritable({ width: 0, height: 0 });
export const robotPosition = currentWritable<LngLat | null>(null);
export const map = currentWritable<Map | null>(null);
export const obstacles = currentWritable<Obstacle[]>([]);
export const waypoints = currentWritable<Waypoint[]>([]);
export const hovered = currentWritable<string | null>(null);

/** The currently selected tab. */
export const tab = persisted<'Obstacles' | 'Waypoints'>('cards.navigation.tab', 'Waypoints');

/** If we're looking at obstacles in a 2d top-down or 3d orbiting view */
export const view = currentWritable<'2D' | '3D'>('2D');

/** Whether or not we can create obstacles */
export const write = currentWritable(false);

/** The bounding radius of an obstacle mapped to obstacle name. */
export const boundingRadius: Record<string, number> = {};

/** The projection matrix of the map camera. */
export const cameraMatrix = new THREE.Matrix4();

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

export const centerMap = (center: LngLat, jumpTo: JumpToOptions | boolean = true) => {
  mapCenter.set(center);

  if (jumpTo) {
    map.current?.jumpTo({
      center,
      ...(jumpTo === true ? {} : jumpTo),
    });
  }
};
