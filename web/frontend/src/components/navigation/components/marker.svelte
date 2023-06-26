<script lang='ts'>

import { onDestroy } from 'svelte'
import { Marker } from 'maplibre-gl'
import { map } from '../stores'

export let lng = 0
export let lat = 0
export let scale = 1
export let visible = true
export let color: string | undefined = undefined

const marker = new Marker({ scale, color });

$: {
  marker.setLngLat([lng, lat]);

  if ($map && visible) {
    marker.addTo($map);
  }
}

$: if (!visible) marker.remove();

onDestroy(() => marker.remove());

</script>
