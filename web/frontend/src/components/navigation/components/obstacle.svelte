
<script lang='ts'>

import { T } from '@threlte/core';
import type { Obstacle } from '../types';
import { view } from '../stores';

export let obstacle: Obstacle;

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
      <T.SphereGeometry args={[geometry.r]} />
    {:else if geometry.type === 'capsule'}
      <T.CapsuleGeometry args={[geometry.r, geometry.l, 16, 32]} />
    {/if}
    <T.MeshPhongMaterial color='red' />
  </T.Mesh>
{/each}