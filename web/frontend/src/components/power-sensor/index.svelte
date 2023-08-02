<script lang="ts">

import {PowerSensorClient, type ServiceError } from '@viamrobotics/sdk';
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
    const results = await Promise.all([
      powerSensorClient.getVoltage(),
      powerSensorClient.getCurrent(),
      powerSensorClient.getPower(),
    ] as const);

    voltageValue = results[0]?.[0];
    currentValue = results[1]?.[0];
    powerValue = results[2];
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
    <div class="flex flex-wrap gap-4 text-sm border border-t-0 border-medium p-4">
        <div class="overflow-auto">
          <h3 class="mb-1">voltage (volts)</h3>
          <div class="flex gap-1.5">
            {JSON.stringify(voltageValue)}
          </div>
        </div>
        <div class="overflow-auto">
          <h3 class="mb-1">
           current (amperes)
          </h3>
          <div class="flex gap-1.5">
            {JSON.stringify(currentValue)}
          </div>
        </div>
        <div class="overflow-auto">
          <h3 class="mb-1">
            power (watts)
          </h3>
          <div class="flex gap-1.5">
            {JSON.stringify(powerValue)}
          </div>
        </div>
        
      
    </Collapse>


