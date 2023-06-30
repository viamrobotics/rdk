<svelte:options immutable />

<script lang="ts">

import 'maplibre-gl/dist/maplibre-gl.css';
import { notify } from '@viamrobotics/prime';
import { navigationApi, type ServiceError } from '@viamrobotics/sdk';
import { setMode, type NavigationModes } from '@/api/navigation';
import { mapCenter, centerMap, robotPosition, flyToMap } from './stores';
import Collapse from '@/lib/components/collapse.svelte';
import Map from './components/map.svelte';
import Nav from './components/nav.svelte';

export let name: string;

const decimalFormat = new Intl.NumberFormat(undefined, { maximumFractionDigits: 7 });

const handleLng = (event: CustomEvent) => {
  const lng = Number.parseFloat(event.detail.value);
  centerMap({ lng, lat: mapCenter.current.lat });
};

const handleLat = (event: CustomEvent) => {
  const lat = Number.parseFloat(event.detail.value);
  centerMap({ lat, lng: mapCenter.current.lng });
};

const setNavigationMode = async (event: CustomEvent) => {
  const mode = event.detail.value as 'Manual' | 'Waypoint';

  const navigationMode: NavigationModes = {
    Manual: navigationApi.Mode.MODE_MANUAL,
    Waypoint: navigationApi.Mode.MODE_WAYPOINT,
  }[mode];

  try {
    await setMode(name, navigationMode);
  } catch (error) {
    notify.danger((error as ServiceError).message);
  }
};

</script>

<Collapse title={name}>
  <v-breadcrumbs
    slot="title"
    crumbs="navigation"
  />

  <div class="flex flex-col gap-2 border border-t-0 border-medium p-4">
    <div class='flex items-end justify-between'>
      <div class='flex gap-6'>
        <div class='flex gap-1 items-end'>
          <v-input class='w-16' label='Base' tooltip='Specified in lng, lat' readonly value={$robotPosition?.lng} />
          <v-input class='w-16' readonly value={$robotPosition?.lat} />
          <v-button
            variant='icon'
            icon='center'
            on:click={() => $robotPosition && flyToMap($robotPosition)}
          />
        </div>

        <v-radio
          label="Navigation mode"
          options="Manual, Waypoint"
          on:input={setNavigationMode}
        />
      </div>

      <div class='flex gap-1.5'>
        <v-input
          type='number'
          label='Longitude'
          placeholder='0'
          incrementor='slider'
          value={$mapCenter.lng ? decimalFormat.format($mapCenter.lng) : ''}
          step='0.5'
          class='max-w-[6rem]'
          on:input={handleLng}
        />
        <v-input
          type='number'
          label='Latitude'
          placeholder='0'
          incrementor='slider'
          value={$mapCenter.lat ? decimalFormat.format($mapCenter.lat) : ''}
          step='0.25'
          class='max-w-[6rem]'
          on:input={handleLat}
        />
      </div>
    </div>

    <div class='flex w-full items-stretch gap-2'>
      <Nav {name} />

      <div class='grow'>
        <Map
          {name}
          on:drag={(event) => centerMap(event.detail, false)}
        />
      </div>

    </div>
  </div>
</Collapse>
