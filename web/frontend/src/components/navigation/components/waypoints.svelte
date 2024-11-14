<script lang="ts">
import { notify } from '@viamrobotics/prime';
import { MapLibreMarker, useMapLibre } from '@viamrobotics/prime-blocks';
import { ConnectError } from '@viamrobotics/sdk';
import type { MapMouseEvent } from 'maplibre-gl';
import { useWaypoints } from '../hooks/use-waypoints';
import { tab } from '../stores';

export let name: string;

const { waypoints, addWaypoint, error } = useWaypoints(name);
const { map } = useMapLibre();

const handleAddMarker = async (event: MapMouseEvent) => {
  if (event.originalEvent.button > 0) {
    return;
  }

  try {
    await addWaypoint(event.lngLat);
  } catch (error_) {
    notify.danger((error_ as ConnectError).message);
  }
};

$: if ($tab === 'Waypoints') {
  map.on('click', handleAddMarker);
} else {
  map.off('click', handleAddMarker);
}

$: if ($error) {
  notify.danger($error.message);
}
</script>

{#each $waypoints as waypoint (waypoint.id)}
  <MapLibreMarker
    scale={0.7}
    lng={waypoint.lng}
    lat={waypoint.lat}
  />
{/each}
