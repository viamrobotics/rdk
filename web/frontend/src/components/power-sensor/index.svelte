<script lang="ts">

import { PowerSensorClient, type ServiceError } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import Collapse from '@/lib/components/collapse.svelte';
import { rcLogConditionally } from '@/lib/log';
import { setAsyncInterval } from '@/lib/schedule';
import { useRobotClient, useDisconnect } from '@/hooks/robot-client';

export let name: string;

const { robotClient } = useRobotClient();

const powerSensorClient = new PowerSensorClient($robotClient, name, {
  requestLogger: rcLogConditionally,
});

let voltageValue: number | undefined;
let currentValue: number | undefined;
let powerValue: number | undefined;

let clearInterval: (() => void) | undefined;

const refresh = async () => {
  try {
    const readings = await powerSensorClient.getReadings();
    voltageValue = readings.voltage;
    currentValue = readings.current;
    powerValue = readings.power;
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const handleToggle = (event: CustomEvent<{ open: boolean }>) => {
  if (event.detail.open) {
    refresh();
    clearInterval = setAsyncInterval(refresh, 500);
  } else {
    clearInterval?.();
  }
};

useDisconnect(() => clearInterval?.());

</script>

<Collapse title={name} on:toggle={handleToggle}>
    <v-breadcrumbs slot="title" crumbs="power_sensor" />
    <div class="flex flex-wrap gap-5 text-sm border border-t-0 border-medium p-4">
      {#if voltageValue !== undefined}
        <div class="overflow-auto">
          <small class='block pt-1 text-sm text-subtle-2'> voltage (volts)</small>
          <div class="flex gap-1.5">
            <small class='block pt-1 text-sm text-subtle-1'>  {voltageValue.toFixed(4)} </small>
          </div>
        </div>
        {/if}
        {#if currentValue !== undefined}
        <div class="overflow-auto">
          <small class='block pt-1 text-sm text-subtle-2'>
           current (amps)
          </small>
          <div class="flex gap-1.5">
            <small class='block pt-1 text-sm text-subtle-1'> {currentValue.toFixed(4)}</small>
          </div>
        </div>
        {/if}
        {#if powerValue !== undefined}
        <div class="overflow-auto">
          <small class='block pt-1 text-sm text-subtle-2'>
            power (watts)
          </small>
          <div class="flex gap-1.5">
            <small class='block pt-1 text-sm text-subtle-1'>  {powerValue.toFixed(4)} </small>
          </div>
        </div>
        {/if}
        </div>
    </Collapse>

