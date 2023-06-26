<script lang='ts'>

import type { ServiceError } from '@viamrobotics/sdk';
import { obstacles, waypoints, setLngLat } from './stores';
import { removeWaypoint } from '@/api/navigation';
import { notify } from '@viamrobotics/prime';

export let name: string

const handleClick = (lng: number, lat: number) => {
  setLngLat({ lng, lat }, { flyTo: {} })
}

const handleRemoveWaypoint = async (id: string) => {
  try {
    $waypoints = $waypoints.filter(item => item.id !== id)
    await removeWaypoint(name, id)
  } catch (error) {
    notify.danger((error as ServiceError).message)
  }
}

</script>

<nav class='min-w-[8rem]'>
  <h3 class='text-xs my-2'>Obstacles</h3>
  <ul class='mb-4 font-mono'>
    {#each $obstacles as { location } (`${location.longitude},${location.latitude}`)}
      <li class='flex group'>
        <v-button
          variant='ghost'
          tooltip='{location.longitude}, {location.latitude}'
          on:click={() => handleClick(location.longitude, location.latitude)}
          label='{location.longitude.toFixed(2)}, {location.latitude.toFixed(2)}'
        />
        <v-button
          class='invisible group-hover:visible text-subtle-2'
          variant='icon'
          icon='trash'
          on:click={() => {}}
        />
      </li>
    {/each}
  </ul>

  <h3 class='text-xs my-2'>Waypoints</h3>
  <ul class='font-mono'>
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
