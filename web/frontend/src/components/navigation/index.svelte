<svelte:options immutable />

<script lang="ts">

import 'maplibre-gl/dist/maplibre-gl.css';
import { type ServiceError, navigationApi } from '@viamrobotics/sdk';
import { notify } from '@viamrobotics/prime';
import { IconButton, persisted } from '@viamrobotics/prime-core';
import { NavigationMap, type LngLat } from '@viamrobotics/prime-blocks';
import { getObstacles } from '@/api/navigation';
import { obstacles } from './stores';
import Collapse from '@/lib/components/collapse.svelte';
import LngLatInput from './components/input/lnglat.svelte';
import { inview } from 'svelte-inview';
import Waypoints from './components/waypoints.svelte';
import { useWaypoints } from './hooks/use-waypoints';
import { useNavMode } from './hooks/use-nav-mode';
import { useNavClient } from './hooks/use-nav-client';
import { useBasePose } from './hooks/use-base-pose';
import type { Map } from 'maplibre-gl';

export let name: string;

let map: Map;

const mapPosition = persisted('viam-blocks-navigation-map-center');
const navClient = useNavClient(name);
const { waypoints, addWaypoint, deleteWaypoint } = useWaypoints(name);
const { mode, setMode } = useNavMode(name);
const { pose } = useBasePose(name);

let centered = false;

$: if (map && $pose && !centered && !mapPosition) {
  map.setCenter($pose);
  centered = true;
}

const handleEnter = async () => {
  try {
    $obstacles = await getObstacles(navClient);
  } catch (error) {
    notify.danger((error as ServiceError).message);
  }
};

const handleModeSelect = async (event: CustomEvent<{ value: 'Manual' | 'Waypoint' }>) => {
  try {
    await setMode(({
      Manual: navigationApi.Mode.MODE_MANUAL,
      Waypoint: navigationApi.Mode.MODE_WAYPOINT,
    } as const)[event.detail.value]);
  } catch (error) {
    notify.danger((error as ServiceError).message);
  }
};

const handleAddWaypoint = async (event: CustomEvent<LngLat>) => {
  try {
    await addWaypoint(event.detail);
  } catch (error) {
    notify.danger((error as ServiceError).message);
  }
};

const handleDeleteWaypoint = async (event: CustomEvent<string>) => {
  try {
    await deleteWaypoint(event.detail);
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

  <div
    use:inview
    on:inview_enter={handleEnter}
    class="flex flex-col gap-2 border border-t-0 border-medium"
  >
    <div class='flex flex-wrap gap-y-2 items-end justify-between py-3 px-4'>
      <div class='flex gap-1'>
        <div class='w-80'>
          <LngLatInput readonly label='Base position' lng={$pose?.lng} lat={$pose?.lat}>
            <IconButton
              label='Focus on base'
              icon='image-filter-center-focus'
              on:click={() => {
                if ($pose) {
                  map.flyTo({
                    zoom: 15,
                    duration: 800,
                    curve: 0.1,
                    center: [$pose.lng, $pose.lat],
                  });
                }
              }}
            />
          </LngLatInput>
        </div>
      </div>

      <v-radio
        label="Navigation mode"
        options="Manual, Waypoint"
        selected={{
          [navigationApi.Mode.MODE_UNSPECIFIED]: '',
          [navigationApi.Mode.MODE_MANUAL]: 'Manual',
          [navigationApi.Mode.MODE_WAYPOINT]: 'Waypoint',
        }[$mode ?? navigationApi.Mode.MODE_UNSPECIFIED]}
        on:input={handleModeSelect}
      />
    </div>

    <div class='relative h-[500px] p-4'>
      <NavigationMap
        bind:map
        environment='debug'
        baseGeoPose={$pose}
        waypoints={$waypoints}
        obstacles={$obstacles}
        on:add-waypoint={handleAddWaypoint}
        on:delete-waypoint={handleDeleteWaypoint}
      >
        <Waypoints {name} />
      </NavigationMap>
    </div>
  </div>
</Collapse>
