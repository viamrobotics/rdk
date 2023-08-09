<script lang='ts'>

import { Map, type MapMouseEvent } from 'maplibre-gl';
import { NavigationClient, type ServiceError } from '@viamrobotics/sdk';
import { notify } from '@viamrobotics/prime';
import { setAsyncInterval } from '@/lib/schedule';
import { useRobotClient, useDisconnect } from '@/hooks/robot-client';
import { waypoints, tab } from '../stores';
import MapMarker from './marker.svelte';
import { formatWaypoints } from '@/api/navigation';
import { rcLogConditionally } from '@/lib/log';

export let map: Map;
export let name: string;

const { robotClient } = useRobotClient();
const navClient = new NavigationClient($robotClient, name, { requestLogger: rcLogConditionally });

const handleAddMarker = async (event: MapMouseEvent) => {
  if (event.originalEvent.button > 0) {
    return;
  }

  const { lat, lng } = event.lngLat;
  const location = { latitude: lat, longitude: lng };
  const temp = { lng, lat, id: crypto.randomUUID() };

  try {
    $waypoints = [...$waypoints, temp];
    await navClient.addWayPoint(location);
  } catch (error) {
    notify.danger((error as ServiceError).message);
    $waypoints = $waypoints.filter((item) => item.id !== temp.id);
  }
};

const updateWaypoints = async () => {
  try {
    const response = await navClient.getWayPoints();
    $waypoints = formatWaypoints(response);
  } catch (error) {
    notify.danger((error as ServiceError).message);
  }
};

const clearUpdateWaypointInterval = setAsyncInterval(updateWaypoints, 1000);
updateWaypoints();

useDisconnect(() => clearUpdateWaypointInterval());

$: if ($tab === 'Waypoints') {
  map.on('click', handleAddMarker);
} else {
  map.off('click', handleAddMarker);
}

</script>

{#each $waypoints as waypoint (waypoint.id)}
  <MapMarker scale={0.7} lngLat={waypoint} />
{/each}
