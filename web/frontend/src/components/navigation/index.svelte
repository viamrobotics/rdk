<svelte:options immutable />

<script lang="ts">

import 'maplibre-gl/dist/maplibre-gl.css';
import type { ServiceError } from '@viamrobotics/sdk'
import { notify } from '@viamrobotics/prime';
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

let map: Map

const navClient = useNavClient(name);
const { waypoints, addWaypoint, deleteWaypoint } = useWaypoints(name);
const { mode, setMode } = useNavMode(name);
const { pose } = useBasePose(name);

let centered = false;

$: if (map && $pose && !centered) {
  map.setCenter($pose);
  centered = true;
}

const handleEnter = async () => {
  $obstacles = await getObstacles(navClient);
};

const handleModeSelect = (event: CustomEvent<{ value: 'Manual' | 'Waypoint' }>) => {
  const mode = ({
    Manual: 1,
    Waypoint: 2,
  } as const)[event.detail.value]
  setMode(mode)
}

const handleAddWaypoint = async (event: CustomEvent<LngLat>) => {
  try {
    await addWaypoint(event.detail)
  } catch (error) {
    notify.danger(error as ServiceError)
  }
}

const handleDeleteWaypoint = async (event: CustomEvent<string>) => {
  deleteWaypoint(event.detail)
}

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
            <v-button
              variant='icon'
              icon='image-filter-center-focus'
              on:click={() => $pose && map?.flyTo({
                zoom: 15,
                duration: 800,
                curve: 0.1,
                center: [$pose.lng, $pose.lat],
              })}
            />
          </LngLatInput>
        </div>
      </div>

      <v-radio
        label="Navigation mode"
        options="Manual, Waypoint"
        selected={{
          0: "",
          1: "Manual",
          2: "Waypoint"
        }[$mode ?? 0]}
        on:input={handleModeSelect}
      />
    </div>

    <div class='relative h-[500px] p-4'>
      <NavigationMap
        bind:map={map}
        environment='debug'
        baseGeoPose={$pose}
        waypoints={$waypoints}
        obstacles={[]}
        on:add-waypoint={handleAddWaypoint}
        on:delete-waypoint={handleDeleteWaypoint}
      >
        <Waypoints {name} />
      </NavigationMap>
    </div>
  </div>
</Collapse>
