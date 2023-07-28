<script lang='ts'>

import { onMount } from 'svelte';
import { type LngLat } from '@/api/navigation';
import { obstacles, flyToMap, mapCenter, tab, hovered } from '../../stores';
import { createObstacle } from '../../lib/obstacle';
import ObstaclesTab from './obstacles.svelte';
import WaypointsTab from './waypoints.svelte';

export let name: string;

const handleSelect = (event: CustomEvent<LngLat>) => {
  flyToMap(event.detail);
};

const handleTabSelect = (event: CustomEvent) => {
  $tab = event.detail.value;
};

onMount(() => {
  // @ts-expect-error Debug function.
  window.DEBUG_addObstacles = () => {
    for (let i = 0; i < 100; i += 1) {
      const lng = $mapCenter.lng + ((i % 10) / 6500);
      const lat = $mapCenter.lat + (Math.trunc(i / 10)) / 6500;
      $obstacles = [
        ...$obstacles,
        createObstacle(`Obstacle ${$obstacles.length + 1}`, { lng, lat }),
      ];
    }
  };
});

</script>

<nav class='w-80'>
  <v-tabs
    tabs="Obstacles, Waypoints"
    selected={$tab}
    on:input={handleTabSelect}
  />

  <ul
    on:mouseleave={() => ($hovered = null)}
    class='px-4 py-2 max-h-[520px] overflow-y-scroll'
  >
    {#if $tab === 'Obstacles'}
      <ObstaclesTab on:select={handleSelect} />

    {:else if $tab === 'Waypoints'}
      <WaypointsTab {name} on:select={handleSelect} />
    {/if}
  </ul>
</nav>
