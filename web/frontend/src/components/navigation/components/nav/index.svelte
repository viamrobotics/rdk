<script lang='ts'>

import { onMount } from 'svelte';
import { obstacles, mapCenter, tab, hovered } from '../../stores';
import { createObstacle } from '../../lib/obstacle';
import ObstaclesTab from './obstacles.svelte';
import WaypointsTab from './waypoints.svelte';

export let name: string;

const handleTabSelect = (event: CustomEvent) => {
  $tab = event.detail.value;
};

onMount(() => {
  // @ts-expect-error Debug function.
  window.DEBUG_addObstacles = () => {
    for (let i = 0; i < 100; i += 1) {
      const lng = $mapCenter.lng + ((i % 10) / 6500);
      const lat = $mapCenter.lat + ((Math.trunc(i / 10)) / 6500);
      $obstacles = [
        ...$obstacles,
        createObstacle(`Obstacle ${$obstacles.length + 1}`, { lng, lat }),
      ];
    }
  };
});

</script>

<nav class='w-full sm:w-80'>
  <v-tabs
    tabs="Waypoints, Obstacles"
    selected={$tab}
    on:input={handleTabSelect}
  />

  <ul
    on:mouseleave={() => ($hovered = null)}
    class='px-4 py-2 max-h-64 sm:max-h-[520px] overflow-y-scroll'
  >
    {#if $tab === 'Waypoints'}
      <WaypointsTab {name} />
    {:else if $tab === 'Obstacles'}
      <ObstaclesTab />
    {/if}
  </ul>
</nav>
