import * as THREE from 'three';
import { currentWritable } from '@threlte/core';
import { type JumpToOptions, type FlyToOptions, type Map } from 'maplibre-gl';
import type { LngLat, Waypoint, Obstacle } from '@/api/navigation';

export const tab = currentWritable<'Obstacles' | 'Waypoints'>('Obstacles');
export const mode = currentWritable<'draw' | 'navigate'>('navigate');
export const view = currentWritable<'2D' | '3D'>('2D');
export const write = currentWritable(false);
export const mapCenter = currentWritable<LngLat>({ lng: 0, lat: 0 });
export const mapZoom = currentWritable(0);
export const mapSize = currentWritable({ width: 0, height: 0 });
export const robotPosition = currentWritable<LngLat | null>(null);
export const map = currentWritable<Map | null>(null);
export const obstacles = currentWritable<Obstacle[]>([]);
export const waypoints = currentWritable<Waypoint[]>([]);
export const hovered = currentWritable<string | null>(null);

export const zooms: Record<string, number> = {};
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
