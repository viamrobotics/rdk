
<script lang='ts'>

import { T } from '@threlte/core';
import type { Obstacle } from '../types';
import { view } from '../stores';
import FlatCapsuleGeometry from './flat-capsule-geometry.svelte'

export let obstacle: Obstacle;

$: console.log(obstacle)

</script>

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
        {#if $view === '3D'}
          <T.BoxGeometry args={[geometry.x, geometry.y, geometry.z]} />
        {:else}
          <T.PlaneGeometry args={[geometry.x, geometry.y]} />
        {/if}
      {:else if geometry.type === 'sphere'}
        {#if $view === '3D'}
          <T.SphereGeometry args={[geometry.r]} />
        {:else}
          <T.CircleGeometry args={[geometry.r]} />
        {/if}
      {:else if geometry.type === 'capsule'}
        {#if $view === '3D'}
          <FlatCapsuleGeometry args={[geometry.r, geometry.l]} />
        {:else}
          <T.CapsuleGeometry args={[geometry.r, geometry.l, 16, 32]} />
        {/if}
      {/if}
      <T.MeshPhysicalMaterial color='red' />
    </T.Mesh>
  {/each}
</T.Group>