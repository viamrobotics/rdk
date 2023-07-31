<script lang='ts'>

import { createEventDispatcher } from 'svelte';
import { mapZoom } from '../../stores';
import type { LngLat } from '@/api/navigation';

export let label: string | undefined = undefined;
export let readonly: true | undefined = undefined;
export let lng: number | undefined;
export let lat: number | undefined;

const decimalFormat = new Intl.NumberFormat(undefined, { maximumFractionDigits: 6 });

$: lngRounded = decimalFormat.format(lng ?? 0);
$: latRounded = decimalFormat.format(lat ?? 0);

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
    value={latRounded}
    step={$mapZoom ** 5}
    class='w-full'
    on:input={handleLat}
    {readonly}
  />
  <v-input
    type='number'
    label={label ? '' : 'Longitude'}
    placeholder='0'
    incrementor={readonly ? undefined : 'slider'}
    value={lngRounded}
    step={$mapZoom ** 5}
    class='w-full'
    on:input={handleLng}
    {readonly}
  />
  <slot />
</div>
