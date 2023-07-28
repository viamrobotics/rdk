<script lang='ts'>

import { createEventDispatcher } from 'svelte';
import { mapZoom } from '../../stores';
import type { LngLat } from '@/api/navigation';

export let label: string | undefined = undefined;
export let readonly: true | undefined = undefined;
export let lng = 0;
export let lat = 0;

const decimalFormat = new Intl.NumberFormat(undefined, { maximumFractionDigits: 6 });

$: lngRounded = decimalFormat.format(lng);
$: latRounded = decimalFormat.format(lat);

const dispatch = createEventDispatcher<{ input: LngLat }>();

const handleLng = (event: CustomEvent) => {
  const { value } = event.detail;
  if (!value) return;
  dispatch('input', { lat: lat ?? 0, lng: Number.parseFloat(value) });
};

const handleLat = (event: CustomEvent) => {
  const { value } = event.detail;
  if (!value) return;
  dispatch('input', { lng: lng ?? 0, lat: Number.parseFloat(value) });
};

</script>

<div class='flex gap-1.5 items-end'>
  <v-input
    type='number'
    label={label ? '' : 'Latitude'}
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
    label={label ?? 'Longitude'}
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
