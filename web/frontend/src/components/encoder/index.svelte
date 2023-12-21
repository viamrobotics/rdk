<script lang="ts">

import { encoderApi, type ServiceError } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import { setAsyncInterval } from '@/lib/schedule';
import { getProperties, getPosition, getPositionDegrees, reset } from '@/api/encoder';
import Collapse from '@/lib/components/collapse.svelte';
import { useRobotClient, useConnect } from '@/hooks/robot-client';

export let name: string;

const { robotClient } = useRobotClient();

let properties: encoderApi.GetPropertiesResponse.AsObject | undefined;
let positionTicks: number | undefined;
let positionDegrees: number | undefined;

let expanded = false;
let cancelInterval: (() => void) | undefined;

const refresh = async () => {
  if (!expanded) return

  try {
    const results = await Promise.all([
      getPosition($robotClient, name),
      properties?.angleDegreesSupported ? getPositionDegrees($robotClient, name) : undefined,
    ] as const);

    positionTicks = results[0];
    positionDegrees = results[1];
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const handleResetClick = async () => {
  try {
    await reset($robotClient, name);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const handleToggle = async (event: CustomEvent<{ open: boolean }>) => {
  if (event.detail.open) {
    
  } else {
    cancelInterval?.();
  }
};

const startPolling = async () => {
  try {
    properties = await getProperties($robotClient, name);
    await refresh();
    cancelInterval = setAsyncInterval(refresh, 500);
  } catch (error) {
    displayError(error as ServiceError);
  }
}

useConnect(() => {
  startPolling()
  return () => cancelInterval?.()
})

$: showPositionTicks = properties?.ticksCountSupported ?? (!properties?.ticksCountSupported && !properties?.angleDegreesSupported)
$: showPositionDegrees = properties?.angleDegreesSupported ?? (!properties?.ticksCountSupported && !properties?.angleDegreesSupported)

</script>

<Collapse title={name} on:toggle={handleToggle}>
  <v-breadcrumbs
    slot="title"
    crumbs="encoder"
  />
  <div class="overflow-auto border border-t-0 border-medium p-4 text-left text-sm">
    <table class="bborder-medium table-auto border">
      {#if showPositionTicks}
        <tr>
          <th class="border border-medium p-2">
            Count
          </th>
          <td class="border border-medium p-2">
            {positionTicks?.toFixed(2)}
          </td>
        </tr>
      {/if}

      {#if showPositionDegrees}
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
