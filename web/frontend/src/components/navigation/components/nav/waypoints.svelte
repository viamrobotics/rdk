<script lang='ts'>

import { createEventDispatcher } from 'svelte';
import type { ServiceError } from '@viamrobotics/sdk';
import { notify } from '@viamrobotics/prime';
import { removeWaypoint, type LngLat } from '@/api/navigation';
import { useRobotClient } from '@/hooks/robot-client';
import { waypoints } from '../../stores';

export let name: string;

const dispatch = createEventDispatcher<{ select: LngLat }>();

const { robotClient } = useRobotClient();

const handleRemoveWaypoint = async (id: string) => {
  try {
    $waypoints = $waypoints.filter((item) => item.id !== id);
    await removeWaypoint($robotClient, name, id);
  } catch (error) {
    notify.danger((error as ServiceError).message);
  }
};

</script>

{#if $waypoints.length === 0}
  <li class='text-xs text-subtle-2 font-sans py-2'>Click to add a waypoint.</li>
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
        icon='image-filter-center-focus'
        aria-label="Focus waypoint {index}"
        on:click={() => dispatch('select', waypoint)}
      />
      <v-button
        class='invisible group-hover:visible text-subtle-2'
        variant='icon'
        aria-label="Remove waypoint {index}"
        icon='trash-can-outline'
        on:click={() => handleRemoveWaypoint(waypoint.id)}
      />
    </div>
  </li>
{/each}
