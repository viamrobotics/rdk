<script lang='ts'>

import { notify } from '@viamrobotics/prime';
import { getLocation } from '@/api/navigation';
import type { ServiceError } from '@viamrobotics/sdk';
import { robotPosition, centerMap } from '../stores';
import { setAsyncInterval } from '@/lib/schedule';
import { useRobotClient, useDisconnect } from '@/hooks/robot-client';
import MapMarker from './marker.svelte';

export let name: string;

const { robotClient } = useRobotClient();

let centered = false;

const updateLocation = async () => {
  try {
    const position = await getLocation($robotClient, name);

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
