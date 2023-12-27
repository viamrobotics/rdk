<script lang="ts">

import { type ServiceError } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import Collapse from '@/lib/components/collapse.svelte';
import { setAsyncInterval } from '@/lib/schedule';
import { getVoltage, getCurrent, getPower } from '@/api/power-sensor';
import { useRobotClient, useConnect } from '@/hooks/robot-client';

export let name: string;

const { robotClient } = useRobotClient();

let voltageValue: number | undefined;
let currentValue: number | undefined;
let powerValue: number | undefined;

let expanded = false;

const refresh = async () => {
  if (!expanded) {
    return;
  }

  try {
    const results = await Promise.all([
      getVoltage($robotClient, name),
      getCurrent($robotClient, name),
      getPower($robotClient, name),
    ] as const)

    voltageValue = results[0];
    currentValue = results[1];
    powerValue = results[2];
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const handleToggle = (event: CustomEvent<{ open: boolean }>) => {
  expanded = event.detail.open;
};

useConnect(() => {
  refresh();
  const clearInterval = setAsyncInterval(refresh, 500);
  return () => clearInterval?.();
})

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

