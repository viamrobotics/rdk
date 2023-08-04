<script lang='ts'>

import { onDestroy } from 'svelte';
import { type LngLatLike, Marker } from 'maplibre-gl';
import { map } from '../stores';

export let lngLat: LngLatLike;
export let scale = 1;
export let visible = true;
export let color: string | null = null;

const marker = new Marker({ scale, color: color ?? undefined });
marker.getElement().style.zIndex = '1';

$: {
  marker.setLngLat(lngLat);

  if ($map && visible) {
    marker.addTo($map);
  }
}

$: if (!visible) {
  marker.remove();
}

onDestroy(() => marker.remove());

</script>
