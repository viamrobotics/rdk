<script lang="ts">

import { onMount, onDestroy } from 'svelte';
import { type Client, encoderApi, type ResponseStream, robotApi, type ServiceError } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import { setAsyncInterval } from '@/lib/schedule';
import { getProperties, getPosition, getPositionDegrees, reset } from '@/api/encoder';
import Collapse from '@/components/collapse.svelte';

export let name: string;
export let client: Client;
export let statusStream: ResponseStream<robotApi.StreamStatusResponse> | null;

let properties: encoderApi.GetPropertiesResponse.AsObject | undefined;
let positionTicks: number | undefined;
let positionDegrees: number | undefined;

let cancelInterval: (() => void) | undefined;

const refresh = async () => {
  try {
    const results = await Promise.all([
      getPosition(client, name),
      properties?.angleDegreesSupported ? getPositionDegrees(client, name) : undefined,
    ] as const);

    positionTicks = results[0];
    positionDegrees = results[1];
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const handleResetClick = async () => {
  try {
    await reset(client, name);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const handleToggle = async (event: CustomEvent<{ open: boolean }>) => {
  if (event.detail.open) {
    try {
      properties = await getProperties(client, name);
      refresh();
      cancelInterval = setAsyncInterval(refresh, 500);
    } catch (error) {
      displayError(error as ServiceError);
    }
  } else {
    cancelInterval?.();
  }
};

onMount(() => {
  statusStream?.on('end', () => cancelInterval?.());
});

onDestroy(() => {
  cancelInterval?.();
});

</script>

<Collapse title={name} on:toggle={handleToggle}>
  <v-breadcrumbs
    slot="title"
    crumbs="encoder"
  />
  <div class="overflow-auto border border-t-0 border-medium p-4 text-left text-sm">
    <table class="bborder-medium table-auto border">
      {#if properties?.ticksCountSupported || (!properties?.ticksCountSupported && !properties?.angleDegreesSupported)}
        <tr>
          <th class="border border-medium p-2">
            Count
          </th>
          <td class="border border-medium p-2">
            {positionTicks?.toFixed(2)}
          </td>
        </tr>
      {/if}

      {#if (
        properties?.angleDegreesSupported ||
        (!properties?.ticksCountSupported && !properties?.angleDegreesSupported)
      )}
        <tr>
          <th class="border border-medium p-2">
            Angle (degrees)
          </th>
          <td class="border border-medium p-2">
            {positionDegrees?.toFixed(2)}
          </td>
        </tr>
      {/if}
    </table>

    <div class="mt-2 flex gap-2">
      <v-button
        label="Reset"
        class="flex-auto"
        on:click={handleResetClick}
      />
    </div>
  </div>
</Collapse>
