<script lang='ts'>

import { type MapMouseEvent } from 'maplibre-gl';
import type { ServiceError } from '@viamrobotics/sdk';
import { notify } from '@viamrobotics/prime';
import { useMapLibre, MapLibreMarker } from '@viamrobotics/prime-blocks';
import { tab } from '../stores';
import { useWaypoints } from '../hooks/use-waypoints';

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
    notify.danger((error_ as ServiceError).message);
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
    pose={{ lat: waypoint.lat, lng: waypoint.lng, rotation: 0 }}
  />
{/each}
