<script lang='ts'>

import { Marker } from 'maplibre-gl';
import { notify } from '@viamrobotics/prime';
import { getLocation } from '@/api/navigation';
import type { ServiceError } from '@viamrobotics/sdk';
import { robotPosition, setLngLat } from './stores';
import { setAsyncInterval } from '@/lib/schedule';
import { useDisconnect } from '@/hooks/use-disconnect';
import MapMarker from './components/marker.svelte';

export let name: string;

let centered = false;

const robotMarker = new Marker({ color: 'red' });

const updateLocation = async () => {
  try {
    const position = await getLocation(name);

    if (!centered) {
      setLngLat(position, { center: true });
      centered = true;
    }

    $robotPosition = position;
  } catch (error) {
    notify.danger((error as ServiceError).message);
    $robotPosition = null;
    robotMarker.remove();
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
