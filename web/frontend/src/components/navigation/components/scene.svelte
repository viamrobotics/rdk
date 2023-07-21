<script lang='ts'>

import * as THREE from 'three';
import { onMount } from 'svelte';
import { T, useThrelte } from '@threlte/core';
// import { useRobotClient } from '@/hooks/robot-client';
// import { getObstacles } from '@/api/navigation';
import { obstacles, view } from '../stores';
import Obstacle from './obstacle.svelte';

export let name: string;

// const { robotClient } = useRobotClient();
const { renderer } = useThrelte();

renderer!.autoClear = false;

// This clips against the map so that intersecting objects will not render over the map
$: renderer!.clippingPlanes = $view === '3D'
  ? [new THREE.Plane(new THREE.Vector3(0, 1, 0), -0.1)]
  : [];

$: flat = $view === '2D';

onMount(async () => {
  // const test = await getObstacles($robotClient, name);
});

</script>

<T.AmbientLight intensity={flat ? 2 : 1} />

{#if !flat}
  <T.DirectionalLight matrixAutoUpdate={true} />
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
