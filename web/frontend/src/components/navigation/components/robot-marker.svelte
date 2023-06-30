<script lang='ts'>

import { notify } from '@viamrobotics/prime';
import { getLocation } from '@/api/navigation';
import type { ServiceError } from '@viamrobotics/sdk';
import { robotPosition, centerMap } from '../stores';
import { setAsyncInterval } from '@/lib/schedule';
import { useClient } from '@/hooks/client';
import { useDisconnect } from '@/hooks/use-disconnect';
import MapMarker from './marker.svelte';

export let name: string;

const { client } = useClient();

let centered = false;

const updateLocation = async () => {
  try {
    const position = await getLocation($client, name);

    if (!centered) {
      centerMap(position);
      centered = true;
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
  <MapMarker
    color='red'
    lng={$robotPosition.lng}
    lat={$robotPosition.lat}
  />
{/if}
