
<script lang='ts'>

import { T } from '@threlte/core';
import type { Obstacle } from '../types';
import { zoomLevels } from '../stores';

export let obstacle: Obstacle;

const createZoomForObstacle = (lngLat: Obstacle['location'], geometry: THREE.BufferGeometry) => {
  geometry.computeBoundingSphere();
  $zoomLevels[`${lngLat.longitude},${lngLat.latitude}`] = 100 / geometry.boundingSphere!.radius;
};

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
        <T.BoxGeometry
          args={[geometry.x, geometry.y, geometry.z]}
          on:create={({ ref }) => createZoomForObstacle(obstacle.location, ref)}
        />
      {:else if geometry.type === 'sphere'}
        <T.SphereGeometry
          args={[geometry.r]}
          on:create={({ ref }) => createZoomForObstacle(obstacle.location, ref)}
        />
      {:else if geometry.type === 'capsule'}
        <T.CapsuleGeometry
          args={[geometry.r, geometry.l, 16, 32]}
          on:create={({ ref }) => createZoomForObstacle(obstacle.location, ref)}
        />
      {/if}
      <T.MeshBasicMaterial color='red' />
    </T.Mesh>
  {/each}
</T.Group>