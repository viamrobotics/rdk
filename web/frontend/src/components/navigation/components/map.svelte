<script lang='ts'>

import { onMount } from 'svelte';
import { Map, NavigationControl } from 'maplibre-gl';
import { map, mapZoom, mapCenter, view } from '../stores';
import { style } from '../style';
import ObstacleLayer from './obstacle-layer.svelte';
import Waypoints from './waypoints.svelte';
import RobotMarker from './robot-marker.svelte';

export let name: string;

const minPitch = 0;
const maxPitch = 60;

const handleViewSelect = (event: CustomEvent) => {
  $view = event.detail.value;
};

const handleMove = () => {
  if (!map.current) {
    return;
  }

  mapCenter.set(map.current.getCenter());
  mapZoom.set(map.current.getZoom() / map.current.getMaxZoom());
  console.log(mapZoom.current, map.current.getZoom())
};

onMount(() => {
  const mapInstance = new Map({
    container: 'navigation-map',
    style,
    center: [0, 0],
    zoom: 9,
    antialias: true,
    minPitch,
    maxPitch: minPitch,
  });

  const nav = new NavigationControl({ showZoom: false });
  mapInstance.addControl(nav, 'top-right');

  mapInstance.on('move', handleMove);
  handleMove();

  $map = mapInstance;
});

$: {
  $map?.setMinPitch(minPitch);
  $map?.setMaxPitch($view === '3D' ? maxPitch : minPitch);
}

</script>

<div
  id='navigation-map'
  class="-mr-4 h-[550px] w-full"
/>

<v-radio
  class='absolute bottom-12 right-3'
  options='2D,3D'
  selected={$view}
  on:input={handleViewSelect}
/>

{#if $map}
  <RobotMarker {name} />
  <Waypoints {name} map={$map} />
  <ObstacleLayer {name} map={$map} />
{/if}

<style>
  :global(#navigation-map ~ canvas) {
    display: none;
  }
</style>
