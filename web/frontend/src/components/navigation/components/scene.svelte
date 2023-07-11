<script lang='ts'>

import * as THREE from 'three';
import { T, useThrelte, useRender, type CurrentWritable } from '@threlte/core';
import { type Map } from 'maplibre-gl';
import { obstacles, zoomLevels } from '../stores';
import { createCameraTransform } from '../utils';
import type { Mat4, Obstacle } from '../types';

export let map: Map;
export let viewProjectionMatrix: CurrentWritable<Float32Array | Mat4>;

const { renderer, scene, camera } = useThrelte();

renderer!.autoClear = false;

// This clips against the map so that intersecting objects will not render over the map
renderer!.clippingPlanes = [new THREE.Plane(new THREE.Vector3(0, 1, 0), -0.1)];

const perspective = camera.current as THREE.PerspectiveCamera;
perspective.far = 100_000;

const cameraTransform = createCameraTransform(map);

const createZoomForObstacle = (lngLat: Obstacle['location'], geometry: THREE.BufferGeometry) => {
  geometry.computeBoundingSphere();
  $zoomLevels[`${lngLat.longitude},${lngLat.latitude}`] = 100 / geometry.boundingSphere!.radius;
};

useRender(() => {
  perspective.projectionMatrix
    .fromArray(viewProjectionMatrix.current)
    .multiply(cameraTransform);

  renderer!.resetState();
  renderer!.render(scene, camera.current);

  map.triggerRepaint();
});

</script>

<T.AmbientLight />

<T.Group
  name='world'
  on:create={({ ref }) => {
    // Rotate into Viam's coordinate system
    ref.rotateY(-Math.PI / 2);
    ref.rotateX(-Math.PI / 2);
  }}
>
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
              on:create={({ ref }) => createZoomForObstacle(obstacle.location, ref)}
            />
          {:else if geometry.type === 'sphere'}
            <T.SphereGeometry
              args={[geometry.r]}
              on:create={({ ref }) => createZoomForObstacle(obstacle.location, ref)}
            />
          {:else if geometry.type === 'capsule'}
            <T.CapsuleGeometry
              args={[geometry.r, geometry.l, 16, 32]}
              on:create={({ ref }) => createZoomForObstacle(obstacle.location, ref)}
            />
          {/if}
          <T.MeshBasicMaterial color='red' />
        </T.Mesh>
      {/each}
    </T.Group>
  {/each}
</T.Group>
