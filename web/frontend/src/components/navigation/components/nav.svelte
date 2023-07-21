<script lang='ts'>

import type { ServiceError } from '@viamrobotics/sdk';
import { notify } from '@viamrobotics/prime';
import { removeWaypoint, type LngLat } from '@/api/navigation';
import { useRobotClient } from '@/hooks/robot-client';
import LnglatInput from './lnglat-input.svelte';
import GeometryInputs from './geometry-inputs.svelte';
import { obstacles, waypoints, flyToMap, mapCenter, write, tab, hovered } from '../stores';
import { createObstacle } from '../lib/obstacle';
    import { onMount } from 'svelte';

export let name: string;

const { robotClient } = useRobotClient();

const handleClick = (lng: number, lat: number) => {
  flyToMap({ lng, lat });
};

const handleRemoveWaypoint = async (id: string) => {
  try {
    $waypoints = $waypoints.filter((item) => item.id !== id);
    await removeWaypoint($robotClient, name, id);
  } catch (error) {
    notify.danger((error as ServiceError).message);
  }
};

const handleAddObstacle = () => {
  $obstacles = [
    createObstacle(`Obstacle ${$obstacles.length + 1}`, $mapCenter.lng, $mapCenter.lat),
    ...$obstacles,
  ];
};

const handleLngLatInput = (index: number, event: CustomEvent<LngLat>) => {
  $obstacles[index]!.location.latitude = event.detail.lat;
  $obstacles[index]!.location.longitude = event.detail.lng;
};

const handleDeleteObstacle = (index: number) => {
  $obstacles = $obstacles.filter((_, i) => i !== index);
};

const handleTabSelect = (event: CustomEvent) => {
  $tab = event.detail.value;
};

onMount(() => {
  // @ts-expect-error Debug function.

  window.DEBUG_addObstacles = () => {
    for (let i = 0; i < 100; i += 1) {
      const x = (i % 10) / 6500;
      const y = (Math.trunc(i / 10)) / 6500;
      $obstacles = [
        createObstacle(`Obstacle ${$obstacles.length + 1}`, $mapCenter.lng + x, $mapCenter.lat + y),
        ...$obstacles,
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
      {#if $obstacles.length === 0}
        <li class='text-xs text-subtle-2 font-sans py-2'>None</li>
      {/if}

      {#if $write}
        <v-button
          class='my-4'
          icon='add'
          label='Add'
          on:click={handleAddObstacle}
        />
      {/if}

      {#each $obstacles as { name: obstacleName, location, geometries }, index (index)}
        {#if $write}
          <li class='group mb-8 pl-2 border-l border-l-medium'>
            <div class='flex items-end gap-1.5 pb-2'>
              <v-input class='w-full' label='Name' value={obstacleName} />
              <v-button
                class='invisible group-hover:visible text-subtle-1'
                variant='icon'
                icon='trash'
                on:click={() => handleDeleteObstacle(index)}
              />
            </div>
            <LnglatInput
              lng={location.longitude}
              lat={location.latitude}
              on:input={(event) => handleLngLatInput(index, event)}>
              <v-button
                class='invisible group-hover:visible text-subtle-1'
                variant='icon'
                icon='center'
                on:click={() => flyToMap({ lng: location.longitude, lat: location.latitude })}
              />

            </LnglatInput>

            {#each geometries as _, geoIndex (geoIndex)}
              <GeometryInputs {index} {geoIndex} />
            {/each}
          </li>
        {:else}
          <li
            class='flex group justify-between items-center gap-1.5 border-b border-b-medium last:border-b-0'
            on:mouseenter={() => ($hovered = name)}
          >
            <small>{name}</small>
            <small class='text-subtle-2 opacity-60 group-hover:opacity-100'>
              ({location.longitude.toFixed(4)}, {location.latitude.toFixed(4)})
            </small>
            <v-button
              class='invisible group-hover:visible text-subtle-1'
              variant='icon'
              icon='center'
              on:click={() => handleClick(location.longitude, location.latitude)}
            />
          </li>
        {/if}
      {/each}
    {:else if $tab === 'Waypoints'}
        {#if $waypoints.length === 0}
          <li class='text-xs text-subtle-2 font-sans py-2'>None</li>
        {/if}

        {#each $waypoints as waypoint, index (waypoint.id)}
          <li class='flex group justify-between items-center gap-1.5 border-b'>
            <small>Waypoint {index}</small>
            <small class='text-subtle-2 opacity-60 group-hover:opacity-100'>
              ({waypoint.lng.toFixed(4)}, {waypoint.lat.toFixed(4)})
            </small>
            <div class='flex items-center gap-1.5'>
              <v-button
                class='invisible group-hover:visible text-subtle-1'
                variant='icon'
                icon='center'
                on:click={() => handleClick(waypoint.lng, waypoint.lat)}
              />
              <v-button
                class='invisible group-hover:visible text-subtle-2'
                variant='icon'
                icon='trash'
                on:click={() => handleRemoveWaypoint(waypoint.id)}
              />
            </div>
          </li>
        {/each}
    {/if}
  </ul>
</nav>
