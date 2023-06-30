<script lang="ts">

import { motorApi, MotorClient, type ServiceError } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import { rcLogConditionally } from '@/lib/log';
import Collapse from '@/lib/components/collapse.svelte';
import { useClient } from '@/hooks/use-client';

export let name: string;
export let status: undefined | {
  is_powered?: boolean
  position?: number
  is_moving?: boolean
};

const motorPosFormat = new Intl.NumberFormat(undefined, {
  maximumFractionDigits: 3,
});

const { client } = useClient();

type MovementTypes = 'go' | 'goFor' | 'goTo';

const motorClient = new MotorClient($client, name, {
  requestLogger: rcLogConditionally,
});

let position = 0;
let rpm = 0;
let power = 50;
let revolutions = 0;

let movementType = 'Go';
let direction: -1 | 1 = 1;
let type: MovementTypes = 'go';
let properties: motorApi.GetPropertiesResponse.AsObject | undefined;

const setMovementType = (event: CustomEvent) => {
  movementType = event.detail.value;
  switch (movementType) {
    case 'Go': {
      type = 'go';
      break;
    }
    case 'Go for': {
      type = 'goFor';
      break;
    }
    case 'Go to': {
      type = 'goTo';
      break;
    }
  }
};

const setPosition = (event: CustomEvent) => {
  position = event.detail.value;
};

const setRpm = (event: CustomEvent) => {
  rpm = event.detail.value;
};

const setRevolutions = (event: CustomEvent) => {
  revolutions = event.detail.value;
};

const setPowerSlider = (event: CustomEvent) => {
  power = event.detail.value;
};

const setDirection = (event: CustomEvent) => {
  switch (event.detail.value) {
    case 'Forwards': {
      direction = 1;
      break;
    }
    case 'Backwards': {
      direction = -1;
      break;
    }
    default: {
      direction = 1;
    }
  }
};

const setPower = async () => {
  const powerPct = (power * direction) / 100;
  try {
    await motorClient.setPower(powerPct);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const goFor = async () => {
  try {
    await motorClient.goFor(rpm * direction, revolutions);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const goTo = async () => {
  try {
    await motorClient.goTo(rpm, position);
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const motorRun = () => {
  switch (type) {
    case 'go': {
      return setPower();
    }
    case 'goFor': {
      return goFor();
    }
    case 'goTo': {
      return goTo();
    }
  }
};

const motorStop = async () => {
  try {
    await motorClient.stop();
  } catch (error) {
    displayError(error as ServiceError);
  }
};

const handleToggle = async (event: CustomEvent<{ open: boolean }>) => {
  if (event.detail.open === false) {
    return;
  }

  try {
    properties = await motorClient.getProperties();
  } catch (error) {
    displayError(error as ServiceError);
  }
};

</script>

<Collapse title={name} on:toggle={handleToggle}>
  <v-breadcrumbs crumbs="motor" slot="title" />
  <div class="flex items-center justify-between gap-2" slot="header">
    {#if properties?.positionReporting}
      <v-badge label="Position {motorPosFormat.format(status?.position ?? 0)}" />
    {/if}
    {#if status?.is_powered}
      <v-badge variant="green" label="Running" />
    {:else}
      <v-badge variant="gray" label="Idle" />
    {/if}
    <v-button
      variant="danger"
      icon="stop-circle"
      label="Stop"
      on:click|stopPropagation={motorStop}
    />
  </div>
  <div class="border border-t-0 border-medium p-4">
    <div class='mb-6'>
      <v-radio
        label="Set power"
        options={properties?.positionReporting ? 'Go, Go for, Go to' : 'Go'}
        selected={movementType}
        on:input={setMovementType}
      />
      <small class='text-xs text-subtle-2'>
        {#if movementType === 'Go'}
          Continuously moves
        {:else if movementType === 'Go for'}
          Relative to where the robot is currently
        {:else if movementType === 'Go to'}
          Relative to home
        {/if}
      </small>
    </div>

    <div class="flex flex-col gap-2">
      {#if movementType === 'Go to'}
        <div class='flex gap-2'>
          <v-input
            type="number"
            label="Position in revolutions"
            value={position}
            class="w-36 pr-2"
            on:input={setPosition}
          />
          <v-input
            type="number"
            class="w-36 pr-2"
            label="RPM"
            value={rpm}
            on:input={setRpm}
          />
        </div>
      {:else if movementType === 'Go for'}
        <div class='flex gap-4 items-end'>
          <v-input
            type="number"
            class="w-36"
            label="# in revolutions"
            value={revolutions}
            on:input={setRevolutions}
          />
          <v-radio
            label="Direction of rotation"
            options="Forwards, Backwards"
            selected={direction === 1 ? 'Forwards' : 'Backwards'}
            on:input={setDirection}
          />
          <v-input
            type="number"
            label="RPM"
            class="w-36"
            value={rpm}
            on:input={setRpm}
          />
        </div>
      {:else if movementType === 'Go'}
        <div class='flex gap-4'>
          <v-radio
            label="Direction of rotation"
            options="Forwards, Backwards"
            selected="{direction === 1 ? 'Forwards' : 'Backwards'}"
            on:input={setDirection}
          />
          <div class="w-64">
            <v-slider
              id="power"
              class="ml-2 max-w-xs"
              min="0"
              max="100"
              step="1"
              suffix="%"
              label="Power %"
              value={power}
              on:input={setPowerSlider}
            />
          </div>
        </div>
      {/if}
    </div>

    <div class="flex flex-row-reverse flex-wrap">
      <v-button
        icon="play-circle-filled"
        variant="success"
        label="Run"
        on:click={motorRun}
      />
    </div>
  </div>
</Collapse>
