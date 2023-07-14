<script lang='ts'>

import type { ServiceError } from '@viamrobotics/sdk';
import { notify } from '@viamrobotics/prime';
import { obstacles, waypoints, flyToMap, mapCenter, mode } from '../stores';
import { removeWaypoint } from '@/api/navigation';
import { useRobotClient } from '@/hooks/robot-client';
import LnglatInput from './lnglat-input.svelte';
import type { LngLat } from '@/api/navigation';
import GeometryInputs from './geometry-inputs.svelte';
import { createObstacle } from '../lib/obstacle';

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
  $obstacles = [...$obstacles, createObstacle($mapCenter.lng, $mapCenter.lat)]
}

const handleLngLatInput = (index: number, event: CustomEvent<LngLat>) => {
  $obstacles[index]!.location.latitude = event.detail.lat;
  $obstacles[index]!.location.longitude = event.detail.lng;
}

const handleDeleteObstacle = (index: number) => {
  $obstacles = $obstacles.filter((_, i) => i !== index)
}

</script>

<nav class='min-w-[10rem] mr-2'>
  <h3 class='text-xs py-1.5 mb-1.5 border-b border-light'>Obstacles</h3>
  <ul class={$mode === 'readonly' ? 'font-mono' : ''}>
    {#if $obstacles.length === 0}
      <li class='text-xs text-subtle-2 font-sans py-2'>None</li>
    {/if}

    {#each $obstacles as { location, geometries }, index}
      {#if $mode === 'readonly'}
        <li class='flex group'>
          <v-button
            variant='ghost'
            tooltip='{location.longitude}, {location.latitude}'
            on:click={() => handleClick(location.longitude, location.latitude)}
            label='{location.longitude.toFixed(2)}, {location.latitude.toFixed(2)}'
          />
        </li>
      {:else}
        <li class='group my-2'>
          <LnglatInput
            lng={location.longitude}
            lat={location.latitude}
            on:input={(event) => handleLngLatInput(index, event)}>
            <v-button
              class='invisible group-hover:visible'
              variant='icon'
              icon='center'
              on:click={() => flyToMap({ lng: location.longitude, lat: location.latitude })}
            />
            <v-button
              class='invisible group-hover:visible'
              variant='icon'
              icon='trash'
              on:click={() => handleDeleteObstacle(index)}
            />

          </LnglatInput>
          {#each geometries as _, geoIndex}
            <GeometryInputs {index} {geoIndex} />
          {/each}
        </li>
      {/if}
    {/each}
  </ul>

  {#if $mode === 'readWrite'}
    <v-button
      class='mb-4'
      icon='add'
      label='Add'
      on:click={handleAddObstacle}
    />
  {/if}

  <h3 class='text-xs py-1.5 mb-1.5 border-b border-light'>Waypoints</h3>
  <ul class='font-mono'>
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
</nav>
