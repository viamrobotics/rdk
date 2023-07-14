<svelte:options immutable />

<script lang="ts">

import 'maplibre-gl/dist/maplibre-gl.css';
import { notify } from '@viamrobotics/prime';
import { navigationApi, type ServiceError } from '@viamrobotics/sdk';
import { setMode, type NavigationModes } from '@/api/navigation';
import { mode as modeStore, mapCenter, centerMap, robotPosition, flyToMap } from './stores';
import { useRobotClient } from '@/hooks/robot-client';
import type { Modes } from './types';
import Collapse from '@/lib/components/collapse.svelte';
import Map from './components/map.svelte';
import Nav from './components/nav.svelte';
import LngLatInput from './components/lnglat-input.svelte';

export let name: string;
export let mode: Modes;

const { robotClient } = useRobotClient();

const decimalFormat = new Intl.NumberFormat(undefined, { maximumFractionDigits: 7 });

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

$: $modeStore = mode;

</script>

<Collapse title={name}>
  <v-breadcrumbs
    slot="title"
    crumbs="navigation"
  />

  <div class="flex flex-col gap-2 border border-t-0 border-medium p-4">
    <div class='flex items-end justify-between'>
      <div class='flex gap-1'>
        <LngLatInput readonly label='Base position' lng={$robotPosition?.lng} lat={$robotPosition?.lat}>
          <v-button
            variant='icon'
            icon='center'
            on:click={() => $robotPosition && flyToMap($robotPosition)}
          />
        </LngLatInput>

        <v-radio
          label="Navigation mode"
          options="Manual, Waypoint"
          on:input={setNavigationMode}
        />
      </div>

      <LngLatInput
        lng={$mapCenter.lng ? Number(decimalFormat.format($mapCenter.lng)) : undefined}
        lat={$mapCenter.lat ? Number(decimalFormat.format($mapCenter.lat)) : undefined}
        on:input={(event) => centerMap(event.detail)}
      />
    </div>

    <div class='flex w-full items-stretch gap-2'>
      <Nav {name} />

      <div class='grow'>
        <Map {name} />
      </div>

    </div>
  </div>
</Collapse>
