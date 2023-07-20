
<script lang='ts'>

import * as THREE from 'three'
import { T } from '@threlte/core';
import type { Obstacle } from '../types';
import { view, hovered } from '../stores';

export let obstacle: Obstacle;

let material: THREE.MeshPhongMaterial

// $: {
//   material.color.set($hovered === obstacle.name ? 'hotpink' : 'red')
// }

</script>

{#each obstacle.geometries as geometry, index (index)}
  <T.Mesh
    lnglat={{
      lng: obstacle.location.longitude,
      lat: obstacle.location.latitude,
    }}
  >
    {#if geometry.type === 'box'}
      {#if $view === '3D'}
        <T.BoxGeometry args={[geometry.x, geometry.y, geometry.z]} />
      {:else}
        <T.PlaneGeometry
          args={[geometry.x, geometry.y]}
          on:create={({ ref }) => ref.rotateX(-Math.PI / 2)}
        />
      {/if}
    {:else if geometry.type === 'sphere'}
      {#if $view === '3D'}
        <T.SphereGeometry args={[geometry.r]} />
      {:else}
        <T.CircleGeometry
          args={[geometry.r]}
          on:create={({ ref }) => ref.rotateX(-Math.PI / 2)}
        />
      {/if}
    {:else if geometry.type === 'capsule'}
      <T.CapsuleGeometry args={[geometry.r, geometry.l, 16, 32]} />
    {/if}
    <T.MeshPhongMaterial
      bind:ref={material}
      color={$hovered === obstacle.name ? '#FFD400' : '#FF7D80'}
    />
  </T.Mesh>
{/each}
