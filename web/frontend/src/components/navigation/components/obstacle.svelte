
<script lang='ts'>

import * as THREE from 'three';
import { T } from '@threlte/core';
import type { Obstacle } from '@/api/navigation';
import { view, hovered } from '../stores';

export let obstacle: Obstacle;

let material: THREE.MeshPhongMaterial;

const rotateGeometry = ({ ref }: { ref: THREE.BufferGeometry }) => {
  ref.rotateX(-Math.PI / 2);
};

</script>

{#each obstacle.geometries as geometry, index (index)}
  <T.Mesh lnglat={obstacle.location}>
    {#if geometry.type === 'box'}
      {#if $view === '3D'}
        <T.BoxGeometry args={[geometry.length, geometry.width, geometry.height]} />
      {:else}
        <T.PlaneGeometry
          args={[geometry.length, geometry.width]}
          on:create={rotateGeometry}
        />
      {/if}
    {:else if geometry.type === 'sphere'}
      {#if $view === '3D'}
        <T.SphereGeometry args={[geometry.radius]} />
      {:else}
        <T.CircleGeometry
          args={[geometry.radius]}
          on:create={rotateGeometry}
        />
      {/if}
    {:else if geometry.type === 'capsule'}
      <T.CapsuleGeometry
        args={[geometry.radius, geometry.length, 16, 32]}
        on:create={rotateGeometry}
      />
    {/if}
    <T.MeshPhongMaterial
      bind:ref={material}
      color={$hovered === obstacle.name ? '#FFD400' : '#FF7D80'}
    />
  </T.Mesh>
{/each}
