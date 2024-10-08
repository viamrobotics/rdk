<svelte:options immutable />

<script lang="ts">
import Collapse from '@/lib/components/collapse.svelte';
import { notify } from '@viamrobotics/prime';
import { NavigationMap, type LngLat } from '@viamrobotics/prime-blocks';
import { Button, IconButton, persisted } from '@viamrobotics/prime-core';
import { ConnectError, navigationApi } from '@viamrobotics/sdk';
import type { Map } from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import LngLatInput from './components/input/lnglat.svelte';
import Waypoints from './components/waypoints.svelte';
import { useBasePose } from './hooks/use-base-pose';
import { useNavMode } from './hooks/use-nav-mode';
import { useObstacles } from './hooks/use-obstacles';
import { usePaths } from './hooks/use-paths';
import { useWaypoints } from './hooks/use-waypoints';

export let name: string;

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
          Manual: navigationApi.Mode.MANUAL,
          Waypoint: navigationApi.Mode.WAYPOINT,
        } as const
      )[event.detail.value]
    );
  } catch (error) {
    notify.danger((error as ConnectError).message);
  }
};

const stopNavigation = async (event: MouseEvent) => {
  event.stopPropagation();

  try {
    await setMode(navigationApi.Mode.MANUAL);
  } catch (error) {
    notify.danger((error as ConnectError).message);
  }
};

const handleAddWaypoint = async (event: CustomEvent<LngLat>) => {
  try {
    await addWaypoint(event.detail);
  } catch (error) {
    notify.danger((error as ConnectError).message);
  }
};

const handleDeleteWaypoint = async (event: CustomEvent<string>) => {
  try {
    await deleteWaypoint(event.detail);
  } catch (error) {
    notify.danger((error as ConnectError).message);
  }
};
</script>

<Collapse title={name}>
  <v-breadcrumbs
    slot="title"
    crumbs="navigation"
  />

  <Button
    slot="header"
    variant="danger"
    icon="stop-circle-outline"
    on:click={stopNavigation}
  >
    Stop
  </Button>

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
          [navigationApi.Mode.UNSPECIFIED]: '',
          [navigationApi.Mode.MANUAL]: 'Manual',
          [navigationApi.Mode.WAYPOINT]: 'Waypoint',
          [navigationApi.Mode.EXPLORE]: 'Explore',
        }[$mode ?? navigationApi.Mode.UNSPECIFIED]}
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
</Collapse>
