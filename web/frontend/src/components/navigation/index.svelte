<svelte:options immutable />

<script lang="ts">

import 'maplibre-gl/dist/maplibre-gl.css';
import { notify } from '@viamrobotics/prime';
import { navigationApi, type ServiceError } from '@viamrobotics/sdk';
import { setMode, type NavigationModes } from '@/api/navigation';
import { mapCenter, centerMap, robotPosition, flyToMap, write as writeStore } from './stores';
import { useRobotClient } from '@/hooks/robot-client';
import Collapse from '@/lib/components/collapse.svelte';
import Map from './components/map.svelte';
import Nav from './components/nav/index.svelte';
import LngLatInput from './components/input/lnglat.svelte';

export let name: string;
export let write = false;

$: $writeStore = write;

const { robotClient } = useRobotClient();

const setNavigationMode = async (event: CustomEvent) => {
  const mode = event.detail.value as 'Manual' | 'Waypoint';

  const navigationMode: NavigationModes = {
    Manual: navigationApi.Mode.MODE_MANUAL,
    Waypoint: navigationApi.Mode.MODE_WAYPOINT,
  }[mode];

  try {
    await setMode($robotClient, name, navigationMode);
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

  <div class="flex flex-col gap-2 border border-t-0 border-medium">
    <div class='flex items-end justify-between py-3 px-4'>
      <div class='flex gap-1'>
        <div class='w-80'>
          <LngLatInput readonly label='Base position' lng={$robotPosition?.lng} lat={$robotPosition?.lat}>
            <v-button
              variant='icon'
              icon='center'
              on:click={() => $robotPosition && flyToMap($robotPosition)}
            />
          </LngLatInput>
        </div>
      </div>

      <v-radio
        label="Navigation mode"
        options="Manual, Waypoint"
        on:input={setNavigationMode}
      />

      <LngLatInput
        lng={$mapCenter.lng}
        lat={$mapCenter.lat}
        on:input={(event) => centerMap(event.detail)}
      />
    </div>

    <div class='flex w-full items-stretch'>

      <Nav {name} />

      <div class='grow'>
        <Map {name} />
      </div>

    </div>
  </div>
</Collapse>
