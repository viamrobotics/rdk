
<script lang='ts'>

import * as THREE from 'three';
import { T } from '@threlte/core';
import type { Obstacle } from '@/api/navigation';
import { view, hovered } from '../stores';

export let obstacle: Obstacle;

let material: THREE.MeshPhongMaterial;

</script>

{#each obstacle.geometries as geometry, index (index)}
  <T.Mesh
    name={obstacle.name}
    obstacle={obstacle.name}
    lnglat={obstacle.location}
    on:create={({ ref }) => {
      ref.rotation.y = geometry.pose.orientationVector.th;
    }}
  >
    {#if geometry.type === 'box'}
      {#if $view === '3D'}
        <T.BoxGeometry
          computeBounding={obstacle.name}
          args={[geometry.length, geometry.width, geometry.height]}
        />
      {:else}
        <T.PlaneGeometry
          computeBounding={obstacle.name}
          args={[geometry.length, geometry.width]}
        />
      {/if}
    {:else if geometry.type === 'sphere'}
      {#if $view === '3D'}
        <T.SphereGeometry
          computeBounding={obstacle.name}
          args={[geometry.radius]}
        />
      {:else}
        <T.CircleGeometry
          computeBounding={obstacle.name}
          args={[geometry.radius]}
        />
      {/if}
    {:else if geometry.type === 'capsule'}
      <T.CapsuleGeometry
        computeBounding={obstacle.name}
        args={[geometry.radius, geometry.length, 16, 32]}
      />
    {/if}
    <T.MeshPhongMaterial
      bind:ref={material}
      color={$hovered === obstacle.name ? '#FFD400' : '#FF7D80'}
    />
  </T.Mesh>
{/each}
