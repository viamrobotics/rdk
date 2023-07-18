<script lang='ts'>

import * as THREE from 'three';
import { T, useThrelte, useRender } from '@threlte/core';
import { obstacles, view } from '../stores';
import Obstacle from './obstacle.svelte';

const { renderer } = useThrelte();

THREE.Object3D.DEFAULT_MATRIX_AUTO_UPDATE = false;
THREE.Object3D.DEFAULT_MATRIX_WORLD_AUTO_UPDATE = false;

renderer!.autoClear = false;

// useRender(() => {
//   renderer!.resetState();
// }, { order: 0 })

// This clips against the map so that intersecting objects will not render over the map
$: renderer!.clippingPlanes = $view === '3D'
  ? [new THREE.Plane(new THREE.Vector3(0, 1, 0), -0.1)]
  : [];

$: flat = $view === '2D'

</script>

<T.PerspectiveCamera
  makeDefault={true}
/>

<T.AmbientLight matrixAutoUpdate={true} intensity={flat ? 2 : 1} />

{#if !flat}
  <T.DirectionalLight matrixAutoUpdate={true} on:create={({ ref }) => { ref.updateMatrixWorld(); }} />
{/if}

<T.Group
  name='world'
  on:create={({ ref }) => {
    // Rotate into Viam's coordinate system
    ref.rotateY(-Math.PI / 2);
    ref.rotateX(-Math.PI / 2);
  }}
>
  
</T.Group>

{#each $obstacles as obstacle}
  <Obstacle obstacle={obstacle} />
{/each}
