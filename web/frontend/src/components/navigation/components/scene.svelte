<script lang='ts'>

import * as THREE from 'three';
import { T, useThrelte, useRender, type CurrentWritable } from '@threlte/core';
import { type Map } from 'maplibre-gl';
import { obstacles } from '../stores';
import { createCameraTransform } from '../utils';
import type { Mat4 } from '../types';
import Obstacle from './obstacle.svelte';

export let map: Map;
export let viewProjectionMatrix: CurrentWritable<Float32Array | Mat4>;

const { renderer, scene, camera } = useThrelte();

renderer!.autoClear = false;

// This clips against the map so that intersecting objects will not render over the map
renderer!.clippingPlanes = [new THREE.Plane(new THREE.Vector3(0, 1, 0), -0.1)];

const perspective = camera.current as THREE.PerspectiveCamera;
perspective.far = 100_000;

const cameraTransform = createCameraTransform(map);

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
    <Obstacle obstacle={obstacle} />
  {/each}
</T.Group>
