<script lang='ts'>

import { onMount, createEventDispatcher } from 'svelte';
import { Map } from 'maplibre-gl';
import { type LngLat } from '@/api/navigation';
import { map, mapZoom } from '../stores';
import { style } from '../style';
import ObstacleLayer from './obstacle-layer.svelte';
import Waypoints from './waypoints.svelte';
import RobotMarker from './robot-marker.svelte';

const dispatch = createEventDispatcher<{
  drag: LngLat
  dragstart: LngLat
  dragend: LngLat
}>();

export let name: string;

onMount(() => {
  const mapInstance = new Map({
    container: 'navigation-map',
    style,
    center: [0, 0],
    zoom: 9,
    pitch: 1,
    antialias: true,
    pitchWithRotate: false,
  });

  const setZoom = () => {
    $mapZoom = mapInstance.getZoom() / mapInstance.getMaxZoom();
  }

  mapInstance.on('drag', () => dispatch('drag', mapInstance.getCenter()));
  mapInstance.on('dragstart', () => dispatch('dragstart', mapInstance.getCenter()));
  mapInstance.on('dragend', () => dispatch('dragend', mapInstance.getCenter()));
  mapInstance.on('zoom', () => setZoom());
  setZoom();
  
  $map = mapInstance;
});

</script>

<div
  id='navigation-map'
  class="mb-2 h-[550px] w-full"
/>

{#if $map}
  <RobotMarker {name} />
  <Waypoints {name} map={$map} />
  <ObstacleLayer map={$map} />
{/if}

<style>
  :global(#navigation-map ~ canvas) {
    display: none;
  }
</style>
