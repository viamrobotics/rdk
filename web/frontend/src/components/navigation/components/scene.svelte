<script lang='ts'>

import * as THREE from 'three';
import { T, useThrelte, useRender } from '@threlte/core';
import { type Map } from 'maplibre-gl';
import { obstacles, view, mapCameraViewProjectionMatrix } from '../stores';
import { createCameraTransform } from '../utils';
import Obstacle from './obstacle.svelte';

export let map: Map;

const { renderer, scene } = useThrelte();

renderer!.autoClear = false;
renderer!.autoClearDepth = false;

let camera: THREE.PerspectiveCamera;

const handleResize = () => {
  const { width, height } = map.getCanvas();
  renderer!.setViewport(0, 0, width, height);
}

map.on('resize', handleResize)
handleResize();

useRender(() => {
  const cameraTransform = createCameraTransform(map);
  camera.projectionMatrix
    .fromArray(mapCameraViewProjectionMatrix.current)
    .multiply(cameraTransform);

  renderer!.getContext().disable(renderer!.getContext().SCISSOR_TEST);
  renderer!.render(scene, camera);
  renderer!.resetState();

  map.triggerRepaint();
});

// This clips against the map so that intersecting objects will not render over the map
$: renderer!.clippingPlanes = $view === '3D'
  ? [new THREE.Plane(new THREE.Vector3(0, 1, 0), -0.1)]
  : [];

$: flat = $view === '2D'

</script>

<T.PerspectiveCamera
  bind:ref={camera}
  makeDefault={true}
  matrixAutoUpdate={false}
/>

<T.AmbientLight intensity={flat ? 2 : 1} />

{#if !flat}
  <T.DirectionalLight />
{/if}

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
