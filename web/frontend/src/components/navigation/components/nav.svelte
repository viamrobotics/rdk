<script lang='ts'>

import type { ServiceError } from '@viamrobotics/sdk';
import { notify } from '@viamrobotics/prime';
import { obstacles, waypoints, flyToMap, mapCenter, write } from '../stores';
import { removeWaypoint } from '@/api/navigation';
import { useRobotClient } from '@/hooks/robot-client';
import LnglatInput from './lnglat-input.svelte';
import type { LngLat } from '@/api/navigation';
import GeometryInputs from './geometry-inputs.svelte';
import { createObstacle } from '../lib/obstacle';
import { tab } from '../stores';

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

setTimeout(() => {
  for (let i = 0; i < 100; i += 1) {
    const x = (i % 10) / 6500
    const y = ((i / 10) | 0) / 6500
    const name = `Obstacle ${$obstacles.length + 1}`
    $obstacles = [createObstacle($mapCenter.lng + x, $mapCenter.lat + y, 'box', name), ...$obstacles]
  }
}, 1000);

const handleAddObstacle = () => {
  const name = `Obstacle ${$obstacles.length + 1}`
  $obstacles = [createObstacle($mapCenter.lng, $mapCenter.lat, 'box', name), ...$obstacles]
}

const handleLngLatInput = (index: number, event: CustomEvent<LngLat>) => {
  $obstacles[index]!.location.latitude = event.detail.lat;
  $obstacles[index]!.location.longitude = event.detail.lng;
}

const handleDeleteObstacle = (index: number) => {
  $obstacles = $obstacles.filter((_, i) => i !== index)
}

const handleTabSelect = (event: CustomEvent) => {
  $tab = event.detail.value
}

</script>

<nav class='w-80'>
  <v-tabs
    tabs="Obstacles, Waypoints"
    selected={$tab}
    on:input={handleTabSelect}
  />
  {#if $tab === 'Obstacles'}
    <ul class='px-4 max-h-[520px] overflow-y-scroll'>
      {#if $obstacles.length === 0}
        <li class='text-xs text-subtle-2 font-sans py-2'>None</li>
      {/if}

      {#if write}
        <v-button
          class='my-4'
          icon='add'
          label='Add'
          on:click={handleAddObstacle}
        />
      {/if}

      {#each $obstacles as { name, location, geometries }, index (index)}
        {#if write}
          <li class='group mb-8 pl-2 border-l border-l-medium'>
            <div class='flex items-end gap-1.5 pb-2'>
              <v-input class='w-full' label='Name' value={name} />
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
          <li class='flex group'>
            <v-button
              variant='ghost'
              tooltip='{location.longitude}, {location.latitude}'
              on:click={() => handleClick(location.longitude, location.latitude)}
              label='{location.longitude.toFixed(2)}, {location.latitude.toFixed(2)}'
            />
          </li>
        {/if}
      {/each}
    </ul>
  {:else if $tab === 'Waypoints'}
    <ul class='max-h-[520px] overflow-y-scroll font-mono'>
      {#if $waypoints.length === 0}
        <li class='text-xs text-subtle-2 font-sans py-2'>None</li>
      {/if}

      {#each $waypoints as waypoint (waypoint.id)}
        <li class='flex group'>
          <v-button
            variant='ghost'
            tooltip='{waypoint.lng.toFixed(7)}, {waypoint.lat.toFixed(7)}'
            label='{waypoint.lng.toFixed(2)}, {waypoint.lat.toFixed(2)}'
            on:click={() => handleClick(waypoint.lng, waypoint.lat)}
          />
          <v-button
            class='invisible group-hover:visible text-subtle-2'
            variant='icon'
            icon='trash'
            on:click={() => handleRemoveWaypoint(waypoint.id)}
          />
        </li>
      {/each}
    </ul>
  {/if}
</nav>
