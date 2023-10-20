<script lang="ts">

import { motorApi, MotorClient, type ServiceError } from '@viamrobotics/sdk';
import { displayError } from '@/lib/error';
import { rcLogConditionally } from '@/lib/log';
import Collapse from '@/lib/components/collapse.svelte';
import { useRobotClient } from '@/hooks/robot-client';

export let name: string;
export let status: undefined | {
  is_powered?: boolean
  position?: number
  is_moving?: boolean
};

const motorPosFormat = new Intl.NumberFormat(undefined, {
  maximumFractionDigits: 3,
});

const { robotClient } = useRobotClient();

type MovementTypes = 'go' | 'goFor' | 'goTo';

const motorClient = new MotorClient($robotClient, name, {
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

const setMovementType = (event: CustomEvent<{ value: string}>) => {
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
  const target = event.currentTarget as HTMLInputElement;

  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
  if (event.type === 'blur' && target.value === undefined) {
    position = 0;
  }

  if (target.value === '') {
    return;
  }

  const num = Number.parseFloat(target.value);

  if (Number.isNaN(num)) {
    return;
  }

  position = num;
};

const setRpm = (event: CustomEvent) => {
  const target = event.currentTarget as HTMLInputElement;

  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
  if (event.type === 'blur' && target.value === undefined) {
    rpm = 0;
  }

  if (target.value === '') {
    return;
  }

  const num = Number.parseFloat(target.value);

  if (Number.isNaN(num)) {
    return;
  }

  rpm = num;
};

const setRevolutions = (event: CustomEvent) => {
  const target = event.currentTarget as HTMLInputElement;

  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
  if (event.type === 'blur' && target.value === undefined) {
    revolutions = 0;
  }

  if (target.value === '') {
    return;
  }

  const num = Number.parseFloat(target.value);

  if (Number.isNaN(num)) {
    return;
  }

  revolutions = num;
};

const setPowerSlider = (event: CustomEvent<{ value: number }>) => {
  power = event.detail.value;
};

const setDirection = (event: CustomEvent<{ value: string }>) => {
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

const motorRun = async () => {
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
  if (!event.detail.open) {
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
      icon="stop-circle-outline"
      label="Stop"
      on:click|stopPropagation={motorStop}
    />
  </div>
  <div class="border border-t-0 border-medium p-4">
    <div class='mb-6'>
      <v-radio
        class='inline-block'
        label="Set power"
        options={properties?.positionReporting ? 'Go, Go for, Go to' : 'Go'}
        selected={movementType}
        on:input={setMovementType}
      />
      <small class='block pt-2 text-xs text-subtle-2'>
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
      <div class='flex gap-4 items-end'>
        {#if movementType === 'Go to'}
          <v-input
            type="number"
            label="Position in revolutions"
            placeholder='0'
            value={position}
            class="w-36 pr-2"
            on:input={setPosition}
            on:blur={setPosition}
          />
          <v-input
            type="number"
            class="w-36 pr-2"
            label="RPM"
            placeholder='0'
            value={rpm}
            on:input={setRpm}
          />
        {:else if movementType === 'Go for'}
          <v-input
            type="number"
            class="w-36"
            label="# in revolutions"
            placeholder='0'
            value={revolutions}
            on:input={setRevolutions}
            on:blur={setRevolutions}
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
            placeholder='0'
            class="w-36"
            value={rpm}
            on:input={setRpm}
            on:blur={setRpm}
          />
        {:else if movementType === 'Go'}
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
        {/if}
      </div>
    </div>

    <div class="flex flex-row-reverse flex-wrap">
      <v-button
        icon="play-circle-outline"
        variant="success"
        label="Run"
        on:click={motorRun}
      />
    </div>
  </div>
</Collapse>
