<script lang='ts'>

import { createEventDispatcher } from 'svelte';
import { mapZoom } from '../stores';
import type { LngLat } from '@/api/navigation';

export let label: string | undefined = undefined
export let readonly: true | undefined = undefined
export let lng: number | undefined
export let lat: number | undefined

const dispatch = createEventDispatcher<{ input: LngLat }>()

const handleLng = (event: CustomEvent) => {
  dispatch('input', { lat: lat ?? 0, lng: Number.parseFloat(event.detail.value) })
}

const handleLat = (event: CustomEvent) => {
  dispatch('input', { lng: lng ?? 0, lat: Number.parseFloat(event.detail.value) })
}

</script>

<div class='flex flex-wrap gap-1.5 items-end'>
  {#if label}
    <p class='w-full m-0 text-xs text-subtle-1'>{label}</p>
  {/if}

  <v-input
    type='number'
    label={label ? '' : 'Longitude'}
    placeholder='0'
    incrementor={readonly ? undefined : 'slider'}
    value={lng}
    step={$mapZoom ** 5}
    class='max-w-[6rem]'
    on:input={handleLng}
    {readonly}
  />
  <v-input
    type='number'
    label={label ? '' : 'Latitude'}
    placeholder='0'
    incrementor={readonly ? undefined : 'slider'}
    value={lat}
    step={$mapZoom ** 5}
    class='max-w-[6rem]'
    on:input={handleLat}
    {readonly}
  />
  <slot />
</div>
