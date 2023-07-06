<script lang='ts'>

import { createEventDispatcher } from 'svelte';

export let label = '';
export let readonly = false
export let placeholders = ['0', '0', '0']
export let labels = ['x', 'y', 'z']
export let type: 'integer' | 'number' = 'number';
export let values: number[] = [];
export let step = 0.1;

const dispatch = createEventDispatcher<{ input: number[] }>();

</script>

<div class='flex flex-wrap gap-1.5 items-end'>
  {#each labels as label, index (label)}
    <v-input
      {type}
      {step}
      {label}
      placeholder={placeholders[index]}
      class='max-w-[5.5rem]'
      readonly={readonly ? 'readonly' : undefined}
      value={values[index] ?? ''}
      incrementor={readonly ? '' : 'slider'}
      on:input={(event) => {
        values[index] = event.detail.value
        dispatch('input', values)
      }}
    />
  {/each}
</div>
