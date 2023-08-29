<svelte:options immutable />

<script lang="ts">

import 'maplibre-gl/dist/maplibre-gl.css';
import { notify } from '@viamrobotics/prime';
import { navigationApi, NavigationClient, type ServiceError } from '@viamrobotics/sdk';
import { getObstacles, type NavigationModes } from '@/api/navigation';
import { mapCenter, centerMap, robotPosition, flyToMap, write as writeStore, obstacles, navMode } from './stores';
import { useRobotClient } from '@/hooks/robot-client';
import Collapse from '@/lib/components/collapse.svelte';
import Map from './components/map.svelte';
import Nav from './components/nav/index.svelte';
import LngLatInput from './components/input/lnglat.svelte';
import { inview } from 'svelte-inview';
import { rcLogConditionally } from '@/lib/log';
import { onMount } from 'svelte';

export let name: string;
export let write = false;

$: $writeStore = write;

const { robotClient } = useRobotClient();
const navClient = new NavigationClient($robotClient, name, { requestLogger: rcLogConditionally });

onMount(async () => {
  const currentMode = await navClient.getMode();
  if (currentMode === 1) {
    $navMode = 'Manual';
  } else if (currentMode === 2) {
    $navMode = 'Waypoint';
  }
});

const setNavigationMode = async (event: CustomEvent) => {
  const mode = event.detail.value as 'Manual' | 'Waypoint';

  const navigationMode: NavigationModes = {
    Manual: navigationApi.Mode.MODE_MANUAL,
    Waypoint: navigationApi.Mode.MODE_WAYPOINT,
  }[mode];

  try {
    await navClient.setMode(navigationMode);
  } catch (error) {
    notify.danger((error as ServiceError).message);
  }
};

const handleEnter = async () => {
  $obstacles = await getObstacles(navClient);
};

</script>

<Collapse title={name}>
  <v-breadcrumbs
    slot="title"
    crumbs="navigation"
  />

  <div
    use:inview
    on:inview_enter={handleEnter}
    class="flex flex-col gap-2 border border-t-0 border-medium"
  >
    <div class='flex flex-wrap gap-y-2 items-end justify-between py-3 px-4'>
      <div class='flex gap-1'>
        <div class='w-80'>
          <LngLatInput readonly label='Base position' lng={$robotPosition?.lng} lat={$robotPosition?.lat}>
            <v-button
              variant='icon'
              icon='image-filter-center-focus'
              on:click={() => $robotPosition && flyToMap($robotPosition)}
            />
          </LngLatInput>
        </div>
      </div>

      <v-radio
        label="Navigation mode"
        options="Manual, Waypoint"
        selected={$navMode}
        on:input={setNavigationMode}
      />

      <LngLatInput
        lng={$mapCenter.lng}
        lat={$mapCenter.lat}
        on:input={(event) => centerMap(event.detail)}
      />
    </div>

    <div class='sm:flex w-full items-stretch'>
      <Nav {name} />

      <div class='relative grow'>
        <Map {name} />
      </div>

    </div>
  </div>
</Collapse>
