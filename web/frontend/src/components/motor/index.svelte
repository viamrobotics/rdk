<script lang="ts">

import { onMount } from 'svelte';
import { Client, motorApi, MotorClient, type ServiceError } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import { rcLogConditionally } from '@/lib/log';

const motorPosFormat = new Intl.NumberFormat(undefined, {
  maximumFractionDigits: 3,
});

export let name: string;
export let status: motorApi.Status.AsObject;
export let client: Client;

type MovementTypes = 'go' | 'goFor' | 'goTo';

const motorClient = new MotorClient(client, name, {
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
    case 'Go For': {
      type = 'goFor';
      break;
    }
    case 'Go To': {
      type = 'goTo';
      break;
    }
  }
};

const setPosition = (event: CustomEvent) => {
  position = event.detail.value
}

const setRpm = (event: CustomEvent) => {
  rpm = event.detail.value
}

const setRevolutions = (event: CustomEvent) => {
  revolutions = event.detail.value
}

const setPowerSlider = (event: CustomEvent) => {
  power = event.detail.value
}

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

onMount(async () => {
  try {
    properties = await motorClient.getProperties();
  } catch (error) {
    displayError(error as ServiceError);
  }
});

</script>

<v-collapse title={name}>
  <v-breadcrumbs crumbs="motor" slot="title" />
  <div class="flex items-center justify-between gap-2" slot="header">
    {#if properties?.positionReporting}
      <v-badge label="Position {motorPosFormat.format(status.position)}" />
    {/if}
    {#if status.isPowered}
      <v-badge variant="green" label="Running" />
    {:else if !status.isPowered}
      <v-badge variant="gray" label="Idle" />
    {/if}
    <v-button
      variant="danger"
      icon="stop-circle"
      label="STOP"
      on:click|stopPropagation={motorStop}
    />
  </div>
  <div>
    <div class="border border-t-0 border-medium p-4">
      <v-radio
        label="Set Power"
        options={properties?.positionReporting ? 'Go, Go For, Go To' : 'Go'}
        selected={movementType}
        class="mb-4"
        on:input={setMovementType}
      />
      <div class="flex flex-wrap items-end gap-4">
        {#if movementType === 'Go To'}
          <div class="flex items-center gap-1">
            <span>{movementType}</span>
            <v-tooltip text="Relative to Home">
              <v-icon name="info-outline" />
            </v-tooltip>
          </div>
          <v-input
            type="number"
            label="Position in Revolutions"
            value={position}
            class="w-48 pr-2"
            on:input={setPosition}
          />
          <v-input
            type="number"
            class="w-32 pr-2"
            label="RPM"
            value={rpm}
            on:input={setRpm}
          />
        {:else if movementType === 'Go For'}
          <div class="flex items-center gap-1">
            <span>{movementType}</span>
            <v-tooltip text="Relative to where the robot is currently">
              <v-icon name="info-outline" />
            </v-tooltip>
          </div>
          <v-input
            type="number"
            class="w-32"
            label="# in Revolutions"
            value={revolutions}
            on:input={setRevolutions}
          />
          <v-radio
            label="Direction of Rotation"
            options="Forwards, Backwards"
            selected={direction === 1 ? 'Forwards' : 'Backwards'}
            on:input={setDirection}
          />
          <v-input
            type="number"
            label="RPM"
            class="w-32"
            value={rpm}
            on:input={setRpm}
          />
        {:else if movementType === 'Go'}
          <div class="flex items-center gap-1">
            <span>{movementType}</span>
            <v-tooltip text="Continuously moves">
              <v-icon name="info-outline" />
            </v-tooltip>
          </div>
          <v-radio
            label="Direction of Rotation"
            options="Forwards, Backwards"
            selected="{direction === 1 ? 'Forwards' : 'Backwards'}"
            on:input={setDirection}
          />
          <div class="w-64">
            <v-slider
              id="power"
              class="ml-2 max-w-xs pt-2"
              min="0"
              max="100"
              step="1"
              suffix="%"
              label="Power %"
              value={power}
              on:input={setPowerSlider}
            />
          </div>
        {/if}
      </div>

      <div class="flex flex-row-reverse flex-wrap">
        <v-button
          icon="play-circle-filled"
          variant="success"
          label="RUN"
          on:click={motorRun}
        />
      </div>
    </div>
  </div>
</v-collapse>