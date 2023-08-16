<script lang='ts'>

import * as THREE from 'three';
import { T, useThrelte } from '@threlte/core';
import { obstacles, view } from '../stores';
import Obstacle from './obstacle.svelte';

const { renderer } = useThrelte();
const clippingPlane = new THREE.Plane(new THREE.Vector3(0, 1, 0), -0.1);

$: flat = $view === '2D';

// This clips against the map so that intersecting objects will not render over the map
$: renderer.clippingPlanes = flat ? [] : [clippingPlane];

</script>

<T.AmbientLight intensity={flat ? 1.5 : 1} />

{#if !flat}
  <T.DirectionalLight intensity={1} />
{/if}

{#each $obstacles as obstacle (obstacle.name)}
  <Obstacle {obstacle} />
{/each}
