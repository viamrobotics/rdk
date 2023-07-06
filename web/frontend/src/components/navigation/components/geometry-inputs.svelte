<script lang='ts'>

import { createEventDispatcher } from 'svelte' 
import VectorInput from './vector-input.svelte'
import type { Geometry } from '../types';

export let geometry: Geometry

let type = 'Box'

const dispatch = createEventDispatcher<{ input: Geometry }>()

</script>

<div class='flex flex-col gap-2 my-2'>
  <v-radio
    options="Box, Sphere, Capsule"
    selected="Box"
    on:input={(event) => (type = event.detail.value)}
  />

  {#if type === 'Box'}
    <VectorInput
      label='Dimensions'
      labels={['Length', 'Width', 'Height']}
      on:input={() => dispatch('input', geometry)}
    />
  {:else if type === 'Sphere'}
    <VectorInput
      label='Dimensions'
      labels={['Radius']}
    />
  {/if}
</div>
