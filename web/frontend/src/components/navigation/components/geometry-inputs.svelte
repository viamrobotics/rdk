<script lang='ts'>

import VectorInput from './vector-input.svelte'
import { obstacles } from '../stores';
    import { createGeometry } from '../lib/geometry';

export let index: number
export let geoIndex: number

let type = 'Box'

const handleShapeSelect = (event: CustomEvent) => {
  const type = event.detail.value.toLowerCase()
  $obstacles[index]!.geometries[geoIndex]! = createGeometry(type)
}

const handleDimensionsInput = (event: CustomEvent<number[]>) => {
  const [x = 0, y = 0, z = 0] = event.detail
  const { type } = $obstacles[index]!.geometries[geoIndex]!
  console.log(x, y, z)

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
    options="Box, Sphere, Capsule"
    selected="Box"
    on:input={handleShapeSelect}
  />

  {#if type === 'Box'}
    <VectorInput
      label='Dimensions'
      labels={['Length', 'Width', 'Height']}
      on:input={handleDimensionsInput}
    />
  {:else if type === 'Capsule'}
    <VectorInput
      label='Dimensions'
      labels={['Radius', 'Length']}
      on:input={handleDimensionsInput}
    />
  {:else if type === 'Sphere'}
    <VectorInput
      label='Dimensions'
      labels={['Radius']}
      on:input={handleDimensionsInput}
    />
  {/if}
</div>
