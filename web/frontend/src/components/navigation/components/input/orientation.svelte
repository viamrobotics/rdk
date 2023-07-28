<script lang='ts'>

import type * as THREE from 'three';
import VectorInput from '../vector-input.svelte';
import { OrientationVector } from '@viamrobotics/three';

export let quaternion: THREE.Quaternion;
export let view: '2D' | '3D' = '2D';

const ov = new OrientationVector();

$: ov.setFromQuaternion(quaternion);

const handleOrientationInput = (event: CustomEvent) => {
  console.log(event);
};

</script>

{#if view === '2D'}
  <VectorInput
    labels={['th']}
    values={[ov.th]}
    on:input={handleOrientationInput}
  />
{:else if view === '3D'}
  <VectorInput
    labels={['x', 'y', 'z', 'w']}
    values={[quaternion.x, quaternion.y, quaternion.z, quaternion.w]}
    on:input={handleOrientationInput}
  />
{/if}
