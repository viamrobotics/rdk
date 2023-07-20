<script lang='ts'>

import VectorInput from './vector-input.svelte'
import { obstacles } from '../stores';
import { createGeometry } from '../lib/geometry';

export let index: number
export let geoIndex: number

$: geometry = $obstacles[index]?.geometries[geoIndex]
$: type = geometry?.type

const handleShapeSelect = (event: CustomEvent) => {
  const type = event.detail.value.toLowerCase()
  $obstacles[index]!.geometries[geoIndex]! = createGeometry(type)
}

const handleDimensionsInput = (event: CustomEvent<number[]>) => {
  const [x = 0, y = 0, z = 0] = event.detail

  let dimensions = {}

  switch (type) {
    case 'box': {
      dimensions = { x, y, z }
      break
    }
    case 'sphere': {
      dimensions = { r: x }
      break
    }
    case 'capsule': {
      dimensions = { r: x, l: y }
      break
    }
  }

  $obstacles[index]!.geometries[geoIndex]! = {
    ...$obstacles[index]!.geometries[geoIndex]!,
    ...dimensions,
  }
}

</script>

<div class='flex flex-col gap-2 my-2'>
  <v-radio
    label='Shape'
    options="Box, Sphere, Capsule"
    selected="Box"
    on:input={handleShapeSelect}
  />

  {#if geometry?.type === 'box'}
    <VectorInput
      label='Dimensions'
      labels={['Length (m)', 'Width (m)', 'Height (m)']}
      values={[geometry?.x, geometry?.y, geometry?.z]}
      on:input={handleDimensionsInput}
    />
  {:else if geometry?.type === 'capsule'}
    <VectorInput
      label='Dimensions'
      labels={['Radius (m)', 'Length (m)']}
      on:input={handleDimensionsInput}
    />
  {:else if type === 'sphere'}
    <VectorInput
      label='Dimensions'
      labels={['Radius (m)']}
      on:input={handleDimensionsInput}
    />
  {/if}
</div>
