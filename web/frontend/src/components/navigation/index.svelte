<svelte:options immutable />

<script lang="ts">
import 'maplibre-gl/dist/maplibre-gl.css';
import { type ServiceError, navigationApi } from '@viamrobotics/sdk';
import { notify } from '@viamrobotics/prime';
import { IconButton, persisted } from '@viamrobotics/prime-core';
import { NavigationMap, type LngLat } from '@viamrobotics/prime-blocks';
import LngLatInput from './components/input/lnglat.svelte';
import Waypoints from './components/waypoints.svelte';
import { useWaypoints } from './hooks/use-waypoints';
import { useNavMode } from './hooks/use-nav-mode';
import { useBasePose } from './hooks/use-base-pose';
import type { Map } from 'maplibre-gl';
import { useObstacles } from './hooks/use-obstacles';
import { usePaths } from './hooks/use-paths';

export let name: string;
export let onStop: (callback: () => Promise<void>) => void

let map: Map | undefined;

const mapPosition = persisted('viam-blocks-navigation-map-center');
const { waypoints, addWaypoint, deleteWaypoint } = useWaypoints(name);
const { mode, setMode } = useNavMode(name);
const { pose } = useBasePose(name);
const { obstacles } = useObstacles(name);
const { paths } = usePaths(name);

let centered = false;

$: if (map && $pose && !centered && !$mapPosition) {
  map.setCenter($pose);
  centered = true;
}

const handleModeSelect = async (
  event: CustomEvent<{ value: 'Manual' | 'Waypoint' }>
) => {
  try {
    await setMode(
      (
        {
          Manual: navigationApi.Mode.MODE_MANUAL,
          Waypoint: navigationApi.Mode.MODE_WAYPOINT,
        } as const
      )[event.detail.value]
    );
  } catch (error) {
    notify.danger((error as ServiceError).message);
  }
};

const stopNavigation = async () => {
  try {
    await setMode(navigationApi.Mode.MODE_MANUAL);
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

onStop(stopNavigation)

</script>

<div class="flex flex-col gap-2 border border-t-0 border-medium">
  <div class="flex flex-wrap items-end justify-between gap-y-2 px-4 py-3">
    <div class="flex gap-1">
      <div class="w-80">
        <LngLatInput
          readonly
          label="Base position"
          lng={$pose?.lng}
          lat={$pose?.lat}
        >
          <IconButton
            label="Focus on base"
            icon="image-filter-center-focus"
            on:click={() => {
              if ($pose) {
                map?.flyTo({
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
        [navigationApi.Mode.MODE_EXPLORE]: 'Explore',
      }[$mode ?? navigationApi.Mode.MODE_UNSPECIFIED]}
      on:input={handleModeSelect}
    />
  </div>

  <div class="relative h-[500px] p-4">
    <NavigationMap
      bind:map
      environment="debug"
      baseGeoPose={$pose}
      waypoints={$waypoints}
      obstacles={$obstacles}
      paths={$paths}
      on:add-waypoint={handleAddWaypoint}
      on:delete-waypoint={handleDeleteWaypoint}
    >
      <Waypoints {name} />
    </NavigationMap>
  </div>
</div>
