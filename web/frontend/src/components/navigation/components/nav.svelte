<script lang='ts'>

import type { ServiceError } from '@viamrobotics/sdk';
import { notify } from '@viamrobotics/prime';
import { obstacles, waypoints, flyToMap } from '../stores';
import { removeWaypoint } from '@/api/navigation';
import { useRobotClient } from '@/hooks/robot-client';

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

</script>

<nav class='min-w-[8rem] mr-2'>
  <h3 class='text-xs py-1.5 mb-1.5 border-b border-light'>Obstacles</h3>
  <ul class='mb-4 font-mono'>
    {#if $obstacles.length === 0}
      <li class='text-xs text-subtle-2 font-sans py-2'>None</li>
    {/if}

    {#each $obstacles as { location } (`${location.longitude},${location.latitude}`)}
      <li class='flex group'>
        <v-button
          variant='ghost'
          tooltip='{location.longitude}, {location.latitude}'
          on:click={() => handleClick(location.longitude, location.latitude)}
          label='{location.longitude.toFixed(2)}, {location.latitude.toFixed(2)}'
        />
        <v-button
          class='invisible text-subtle-2'
          variant='icon'
          icon='trash'
          on:click={() => null}
        />
      </li>
    {/each}
  </ul>

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
