<script lang='ts'>

import * as THREE from 'three';
import { T, useThrelte, useRender, type CurrentWritable } from '@threlte/core';
import { type Map } from 'maplibre-gl';
import { obstacles, zoomLevels } from './stores';
import { cameraPerspectiveToOrtho, createCameraTransform } from './utils';
import type { Obstacle } from './types';

export let map: Map;
export let viewProjectionMatrix: CurrentWritable<Float32Array | [number, number, number, number, number, number, number, number, number, number, number, number, number, number, number, number]>;

const view: 'orthographic' | 'perspective' = 'perspective';

const { renderer, scene, camera } = useThrelte();

renderer!.autoClear = false;

// This clips against the map so that interesecting objects will not render over the map
renderer!.clippingPlanes = [new THREE.Plane(new THREE.Vector3(0, 1, 0), -0.1)];

const perspective = camera.current as THREE.PerspectiveCamera;
const orthographic = new THREE.OrthographicCamera(-1, 1, 1, -1, 0.1, 100_000);
perspective.far = 100_000;

const cameraTransform = createCameraTransform(map);

const setZoomLevel = (lngLat: Obstacle['location'], geometry: THREE.BufferGeometry) => {
  geometry.computeBoundingSphere();
  $zoomLevels[`${lngLat.longitude},${lngLat.latitude}`] = 100 / geometry.boundingSphere!.radius;
};

useRender(() => {
  perspective.projectionMatrix
    .fromArray(viewProjectionMatrix.current)
    .multiply(cameraTransform);

  cameraPerspectiveToOrtho(perspective, orthographic);

  renderer!.resetState();
  renderer!.render(scene, camera.current);

  map.triggerRepaint();
});

</script>

<T.AmbientLight />

<T
  is={orthographic}
  makeDefault={view === 'orthographic'}
/>

{#each $obstacles as obstacle}
  <T.Group lnglat={{
    lng: obstacle.location.longitude,
    lat: obstacle.location.latitude,
  }}>
    {#each obstacle.geometries as geometry}
      <T.Mesh
        position.x={geometry.translation.x}
        position.y={geometry.translation.y}
        position.z={geometry.translation.z}
      >
        {#if geometry.type === 'box'}
          <T.BoxGeometry
            args={[geometry.x, geometry.y, geometry.z]}
            on:create={({ ref }) => setZoomLevel(obstacle.location, ref)}
          />
        {:else if geometry.type === 'sphere'}
          <T.SphereGeometry
            args={[geometry.r]}
            on:create={({ ref }) => ref.computeBoundingSphere()}
          />
        {:else if geometry.type === 'capsule'}
          <T.CapsuleGeometry
            args={[geometry.r, geometry.l, 4, 8]}
            on:create={({ ref }) => {
              ref.computeBoundingSphere();
              ref.rotateX(Math.PI / 2);
            }}
          />
        {/if}
        <T.MeshBasicMaterial color='red' />
      </T.Mesh>
    {/each}
  </T.Group>
{/each}
