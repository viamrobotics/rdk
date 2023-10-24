<script lang='ts'>

import { onMount } from 'svelte';
import { Map, NavigationControl } from 'maplibre-gl';
import { map, mapZoom, mapCenter, view, mapSize, cameraMatrix } from '../stores';
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
  $map = mapInstance;

  const handleMove = () => {
    mapCenter.set(mapInstance.getCenter());
    mapZoom.set(mapInstance.getZoom() / mapInstance.getMaxZoom());
  };

  const handleResize = () => {
    mapSize.update((value) => {
      const { clientWidth, clientHeight } = mapInstance.getCanvas();
      value.width = clientWidth;
      value.height = clientHeight;
      return value;
    });
  };

  const nav = new NavigationControl({ showZoom: false });
  mapInstance.addControl(nav, 'top-right');
  mapInstance.on('move', handleMove);
  mapInstance.on('resize', handleResize);

  mapInstance.on('style.load', () => {
    mapInstance.addLayer({
      id: 'obstacle-layer',
      type: 'custom',
      renderingMode: '3d',
      render (_ctx, viewProjectionMatrix) {
        cameraMatrix.fromArray(viewProjectionMatrix);
        mapInstance.triggerRepaint();
      },
    });
  });

  handleMove();
  handleResize();

  return () => {
    if (mapInstance.getLayer('obstacle-layer')) {
      mapInstance.removeLayer('obstacle-layer');
    }
  };
});

$: {
  $map?.setMinPitch(minPitch);
  $map?.setMaxPitch($view === '3D' ? maxPitch : minPitch);
}

</script>

<div
  id='navigation-map'
  class="-mr-4 h-[450px] sm:h-[550px] w-full"
/>

{#if localStorage.getItem('debug_3d')}
  <v-radio
    class='absolute bottom-12 right-3'
    options='2D,3D'
    selected={$view}
    on:input={handleViewSelect}
  />
{/if}

{#if $map}
  <RobotMarker {name} />
  <Waypoints {name} map={$map} />
  <ObstacleLayer />
{/if}
