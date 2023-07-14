import * as THREE from 'three'
import { currentWritable } from '@threlte/core';
import { type JumpToOptions, type FlyToOptions, type Map } from 'maplibre-gl';
import type { Modes, Views, Obstacle, Mat4 } from './types';
import type { LngLat, Waypoint } from '@/api/navigation';

export const view = currentWritable<Views>('3D');
export const mode = currentWritable<Modes>('readonly');
export const mapCenter = currentWritable<LngLat>({ lng: 0, lat: 0 });
export const mapCameraViewProjectionMatrix = currentWritable<Float32Array | Mat4>(null!);
export const mapZoom = currentWritable<number>(0);
export const robotPosition = currentWritable<LngLat | null>(null);
export const map = currentWritable<Map | null>(null);
export const obstacles = currentWritable<Obstacle[]>([]);
export const waypoints = currentWritable<Waypoint[]>([]);

export const mapCamera = new THREE.PerspectiveCamera();

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
