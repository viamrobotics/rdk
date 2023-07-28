<script lang='ts'>
import { createEventDispatcher } from 'svelte';
import type { Geometry } from '@/api/navigation';
import VectorInput from '../vector-input.svelte';
import { createGeometry } from '../../lib/geometry';

export let geometry: Geometry;

const dispatch = createEventDispatcher<{ input: Geometry }>();

const handleShapeSelect = (event: CustomEvent) => {
  const nextType = event.detail.value.toLowerCase();
  dispatch('input', createGeometry(nextType));
};

const handleDimensionsInput = (event: CustomEvent<number[]>) => {
  const [x, y, z] = event.detail;

  switch (geometry.type) {
    case 'box': {
      if (x) {
        geometry.length = x;
      }
      if (y) {
        geometry.width = y;
      }
      if (z) {
        geometry.height = z;
      }
      break;
    }
    case 'sphere': {
      if (x) {
        geometry.radius = x;
      }
      break;
    }
    case 'capsule': {
      if (x) {
        geometry.radius = x;
      }
      if (y) {
        geometry.length = y;
      }
      break;
    }
  }

  dispatch('input', geometry);
};

const shapeMap = { box: 'Box', sphere: 'Sphere', capsule: 'Capsule' };

</script>

<div class='flex flex-col gap-2 my-2'>
  <v-radio
    label='Shape'
    options="Box, Sphere, Capsule"
    selected={shapeMap[geometry.type]}
    on:input={handleShapeSelect}
  />

  {#if geometry.type === 'box'}
    <VectorInput
      labels={['Length (m)', 'Width (m)', 'Height (m)']}
      values={[geometry.length, geometry.width, geometry.height]}
      on:input={handleDimensionsInput}
    />
  {:else if geometry.type === 'capsule'}
    <VectorInput
      labels={['Radius (m)', 'Length (m)']}
      values={[geometry.radius, geometry.length]}
      on:input={handleDimensionsInput}
    />
  {:else if geometry.type === 'sphere'}
    <VectorInput
      labels={['Radius (m)']}
      values={[geometry.radius]}
      on:input={handleDimensionsInput}
    />
  {/if}
</div>
