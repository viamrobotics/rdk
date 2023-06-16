<svelte:options immutable />

<script lang="ts">

import 'maplibre-gl/dist/maplibre-gl.css';
import { notify } from '@viamrobotics/prime';
import { Client, robotApi, navigationApi, type ServiceError, type ResponseStream } from '@viamrobotics/sdk';
import { setMode, type NavigationModes } from '@/api/navigation';

import Collapse from '@/components/collapse.svelte';
import Map from './map.svelte';

export let name: string;
export let client: Client;
export let statusStream: ResponseStream<robotApi.StreamStatusResponse> | null;

const setNavigationMode = async (event: CustomEvent) => {
  const mode = event.detail.value as 'Manual' | 'Waypoint' | 'Unspecified';

  const navigationMode: NavigationModes = {
    Manual: navigationApi.Mode.MODE_MANUAL,
    Waypoint: navigationApi.Mode.MODE_WAYPOINT,
    Unspecified: navigationApi.Mode.MODE_UNSPECIFIED,
  }[mode];

  try {
    await setMode(client, name, navigationMode);
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
    <v-radio
      label="Navigation mode"
      options="Manual, Waypoint, Unspecified"
      selected="Unspecified"
      on:input={setNavigationMode}
    />

    <Map
      {name}
      {client}
      {statusStream}
    />
  </div>
</Collapse>
