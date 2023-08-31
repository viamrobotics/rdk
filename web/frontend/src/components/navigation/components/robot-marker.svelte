<script lang='ts'>

import { notify } from '@viamrobotics/prime';
import { NavigationClient, type ServiceError } from '@viamrobotics/sdk';
import { robotPosition, centerMap } from '../stores';
import { setAsyncInterval } from '@/lib/schedule';
import { useRobotClient, useDisconnect } from '@/hooks/robot-client';
import MapMarker from './marker.svelte';
import { rcLogConditionally } from '@/lib/log';

export let name: string;

const { robotClient } = useRobotClient();
const navClient = new NavigationClient($robotClient, name, { requestLogger: rcLogConditionally });

let centered = false;

const updateLocation = async () => {
  try {
    const { location } = await navClient.getLocation();
    if (!location) {
      return;
    }
    const { latitude, longitude } = location;

    const position = { lat: latitude, lng: longitude };
    if (!centered) {
      centerMap(position, true);
      centered = true;
    }

    if ($robotPosition?.lat === position.lat && $robotPosition.lng === position.lng) {
      return;
    }

    $robotPosition = position;
  } catch (error) {
    notify.danger((error as ServiceError).message);
    $robotPosition = null;
  }
};

updateLocation();
const clearUpdateLocationInterval = setAsyncInterval(updateLocation, 300);

useDisconnect(() => clearUpdateLocationInterval());

</script>

{#if $robotPosition}
  <MapMarker color='#01EF83' lngLat={$robotPosition} />
{/if}
