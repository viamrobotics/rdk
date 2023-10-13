<script lang='ts'>

import { createEventDispatcher } from 'svelte';
import type { LngLat } from '@/api/navigation';

export let label: string | undefined = undefined;
export let readonly: true | undefined = undefined;
export let lng: number | undefined;
export let lat: number | undefined;

const dispatch = createEventDispatcher<{ input: LngLat }>();

const handleLng = (event: CustomEvent<{ value: string }>) => {
  const { value } = event.detail;
  if (!value) {
    return;
  }
  dispatch('input', { lat: lat ?? 0, lng: Number.parseFloat(value) });
};

const handleLat = (event: CustomEvent<{ value: string }>) => {
  const { value } = event.detail;
  if (!value) {
    return;
  }
  dispatch('input', { lng: lng ?? 0, lat: Number.parseFloat(value) });
};

</script>

<div class='flex gap-1.5 items-end'>
  <v-input
    type='number'
    label={label ?? 'Latitude'}
    placeholder='0'
    incrementor={readonly ? undefined : 'slider'}
    value={lat}
    on:input={handleLat}
    {readonly}
  />
  <v-input
    type='number'
    label={label ? '' : 'Longitude'}
    placeholder='0'
    incrementor={readonly ? undefined : 'slider'}
    value={lng}
    on:input={handleLng}
    {readonly}
  />
  <slot />
</div>
