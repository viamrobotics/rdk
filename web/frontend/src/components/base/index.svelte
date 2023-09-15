<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { BaseClient, type ServiceError } from '@viamrobotics/sdk';
  import { filterSubtype } from '../../lib/resource';
  import { displayError } from '../../lib/error';
  import KeyboardInput from '../keyboard-input/index.svelte';
  import type { Keys } from '../keyboard-input/types';
  import Camera from '../camera/camera.svelte';
  import { rcLogConditionally } from '../../lib/log';
  import { selectedMap } from '../../lib/camera-state';
  import { clickOutside } from '../../lib/click-outside';
  import Collapse from '@/lib/components/collapse.svelte';
  import { components } from '@/stores/resources';
  import { useRobotClient } from '@/hooks/robot-client';

  export let name: string;

  const enum Keymap {
    LEFT = 'a',
    RIGHT = 'd',
    FORWARD = 'w',
    BACKWARD = 's',
  }

  type Tabs = 'Keyboard' | 'Discrete';
  type MovementTypes = 'Continuous' | 'Discrete';
  type MovementModes = 'Straight' | 'Spin';
  type SpinTypes = 'Clockwise' | 'Counterclockwise';
  type Directions = 'Forwards' | 'Backwards';
  type View = 'Stacked' | 'Grid';

  const { robotClient } = useRobotClient();

  const baseClient = new BaseClient($robotClient, name, {
    requestLogger: rcLogConditionally,
  });

  let refreshFrequency = 'Live';
  let triggerRefresh = false;

  const openCameras: Record<string, boolean | undefined> = {};
  let selectedView: View = 'Stacked';
  let selectedMode: Tabs = 'Keyboard';
  let movementMode: MovementModes = 'Straight';
  let movementType: MovementTypes = 'Continuous';
  let direction: Directions = 'Forwards';
  let spinType: SpinTypes = 'Clockwise';
  let disableRefresh = true;
  let disableViews = true;

  let increment = 1000;
  // straight mm/s
  let speed = 300;
  // deg/s
  let spinSpeed = 90;
  let angle = 180;
  let power = 50;

  const pressed = new Set<Keys>();
  let stopped = true;

  let isKeyboardActive = false;

  $: cameras = filterSubtype($components, 'camera');

  const resetDiscreteState = () => {
    movementMode = 'Straight';
    movementType = 'Continuous';
    direction = 'Forwards';
    spinType = 'Clockwise';
  };

  const setMovementMode = (mode: MovementModes) => {
    movementMode = mode;
  };

  const setSpinType = (type: SpinTypes) => {
    spinType = type;
  };

  const stop = async () => {
    stopped = true;

    try {
      await baseClient.stop();
    } catch (error) {
      displayError(error as ServiceError);
    }
  };

  const digestInput = async () => {
    let linearValue = 0;
    let angularValue = 0;

    for (const item of pressed) {
      switch (item) {
        case Keymap.FORWARD: {
          linearValue += Number(0.01 * power);
          break;
        }
        case Keymap.BACKWARD: {
          linearValue -= Number(0.01 * power);
          break;
        }
        case Keymap.LEFT: {
          angularValue += Number(0.01 * power);
          break;
        }
        case Keymap.RIGHT: {
          angularValue -= Number(0.01 * power);
          break;
        }
      }
    }

    const linear = { x: 0, y: linearValue, z: 0 };
    const angular = { x: 0, y: 0, z: angularValue };
    try {
      await baseClient.setPower(linear, angular);
    } catch (error) {
      displayError(error as ServiceError);
    }

    if (pressed.size <= 0) {
      stop();
    }
  };

  const handleKeyDown = (key: Keys) => {
    pressed.add(key);
    digestInput();
  };

  const handleKeyUp = (key: Keys) => {
    pressed.delete(key);

    if (pressed.size > 0) {
      stopped = false;
      digestInput();
    } else {
      stop();
    }
  };

  const handleBaseStraight = async (event: {
    distance: number;
    speed: number;
    direction: number;
    movementType: MovementTypes;
  }) => {
    if (event.movementType === 'Continuous') {
      const linear = { x: 0, y: event.speed * event.direction, z: 0 };
      const angular = { x: 0, y: 0, z: 0 };

      try {
        await baseClient.setVelocity(linear, angular);
      } catch (error) {
        displayError(error as ServiceError);
      }
    } else {
      try {
        await baseClient.moveStraight(
          event.distance,
          event.speed * event.direction
        );
      } catch (error) {
        displayError(error as ServiceError);
      }
    }
  };

  const baseRun = async () => {
    if (movementMode === 'Spin') {
      try {
        await baseClient.spin(
          angle * (spinType === 'Clockwise' ? -1 : 1),
          spinSpeed
        );
      } catch (error) {
        displayError(error as ServiceError);
      }
    } else if (movementMode === 'Straight') {
      handleBaseStraight({
        movementType,
        direction: direction === 'Forwards' ? 1 : -1,
        speed,
        distance: increment,
      });
    }
  };

  const handleViewSelect = (event: CustomEvent) => {
    selectedView = event.detail.value;

    let liveCameras = 0;
    for (const camera of cameras) {
      if (openCameras[camera.name]) {
        liveCameras += 1;
      }
    }
    disableViews = liveCameras > 1;
  };

  const handleOnBlur = () => {
    if (pressed.size <= 0) {
      pressed.clear();
      stop();
    }
  };

  const handleVisibilityChange = () => {
    // only issue the stop if there are keys pressed as to not interfere with commands issued by scripts/commands
    if (document.visibilityState === 'hidden') {
      handleOnBlur();
    }
  };

  const handleUpdateKeyboardState = (on: boolean) => {
    isKeyboardActive = on;

    if (isKeyboardActive) {
      return;
    }

    if (pressed.size > 0 || !stopped) {
      stop();
    }
  };

  const handleSwitch = (cameraName: string) => {
    openCameras[cameraName] = !openCameras[cameraName];

    for (const camera of cameras) {
      if (openCameras[camera.name]) {
        disableRefresh = false;
        return;
      }
    }
    disableRefresh = true;
  };

  const handleSelectMovementMode = (event: CustomEvent) => {
    setMovementMode(event.detail.value);
  };

  const handleControlModeSelect = (event: CustomEvent) => {
    selectedMode = event.detail.value;

    if (selectedMode === 'Discrete') {
      resetDiscreteState();
    }
  };

  const handleSelectMovementType = (event: CustomEvent) => {
    movementType = event.detail.value;
  };

  const handlePowerSlider = (event: CustomEvent) => {
    power = event.detail.value;
  };

  const handleSetDirection = (event: CustomEvent) => {
    direction = event.detail.value;
  };

  const handleSetSpeed = (event: CustomEvent) => {
    speed = event.detail.value;
  };

  const handleSetIncrement = (event: CustomEvent) => {
    increment = event.detail.value;
  };

  const handleSetSpinSpeed = (event: CustomEvent) => {
    spinSpeed = event.detail.value;
  };

  const handleSetSpinType = (event: CustomEvent) => {
    setSpinType(event.detail.value);
  };

  const handleSetRefreshFrequency = (event: CustomEvent) => {
    refreshFrequency = event.detail.value;
  };

  const handleSetAngle = (event: CustomEvent) => {
    angle = event.detail.value;
  };

  const handleOnKeyUpRun = (event: KeyboardInput) => {
    if (event.key === 13) {
      baseRun();
    }
  };

  const handleOnKeyUpStop = (event: KeyboardInput) => {
    if (event.key === 13) {
      stop();
    }
  };

  onMount(() => {
    window.addEventListener('visibilitychange', handleVisibilityChange);

    // Safety measure for system prompts, etc.
    window.addEventListener('blur', handleOnBlur);

    for (const camera of cameras) {
      openCameras[camera.name] = false;
    }
  });

  onDestroy(() => {
    handleOnBlur();

    window.removeEventListener('visibilitychange', handleVisibilityChange);
    window.removeEventListener('blur', handleOnBlur);
  });
</script>

<div use:clickOutside={() => {
  isKeyboardActive = false;
}}>
  <Collapse title={name}>
    <v-breadcrumbs slot="title" crumbs="base" />

    <v-button
      slot="header"
      variant="danger"
      icon="stop-circle-outline"
      label="Stop"
      on:click={stop}
      on:keyup={handleOnKeyUpStop}
      role="button"
      tabindex="0"
    />

    <div
      class="flex flex-wrap gap-4 border border-t-0 border-medium sm:flex-nowrap"
    >
      <div class="flex min-w-fit flex-col gap-4 p-4">
        <h2 class="font-bold">Motor controls</h2>
        <v-radio
          label="Control mode"
          options="Keyboard, Discrete"
          selected={selectedMode}
          on:input={handleControlModeSelect}
        />

        {#if selectedMode === 'Keyboard'}
          <div>
            <KeyboardInput
              isActive={isKeyboardActive}
              onKeyDown={handleKeyDown}
              onKeyUp={handleKeyUp}
              onUpdateKeyboardState={handleUpdateKeyboardState}
            />
            <v-slider
              id="power"
              class="w-full max-w-xs pt-2"
              min={0}
              max={100}
              step={1}
              suffix="%"
              label="Power %"
              value={power}
              on:input={handlePowerSlider}
            />
          </div>
        {/if}

        {#if selectedMode === 'Discrete'}
          <div class="flex flex-col gap-4">
            <v-radio
              label="Movement mode"
              options="Straight, Spin"
              selected={movementMode}
              on:input={handleSelectMovementMode}
            />
            {#if movementMode === 'Straight'}
              <v-radio
                label="Movement type"
                options="Continuous, Discrete"
                selected={movementType}
                on:input={handleSelectMovementType}
              />
              <v-radio
                label="Direction"
                options="Forwards, Backwards"
                selected={direction}
                on:input={handleSetDirection}
              />
              <v-input
                type="number"
                value={speed}
                label="Speed (mm/sec)"
                on:input={handleSetSpeed}
              />
              {#if movementType === 'Continuous'}
                <v-input
                  type="number"
                  value={increment}
                  tabindex="-1"
                  disabled
                  label="Distance (mm)"
                />
              {:else}
                <v-input
                  type="number"
                  value={increment}
                  label="Distance (mm)"
                  on:input={handleSetIncrement}
                />
              {/if}
            {/if}
            {#if movementMode === 'Spin'}
              <v-input
                type="number"
                value={spinSpeed}
                label="Speed (deg/sec)"
                on:input={handleSetSpinSpeed}
              />
              <v-radio
                label="Movement Type"
                options="Clockwise, Counterclockwise"
                selected={spinType}
                on:input={handleSetSpinType}
              />
              <div>
                <v-slider
                  min={0}
                  max={360}
                  step={15}
                  suffix="°"
                  label="Angle"
                  value={angle}
                  on:input={handleSetAngle}
                />
              </div>
            {/if}
            <v-button
              icon="play-circle-outline"
              variant="success"
              label="Run"
              on:click={baseRun}
              on:keyup={handleOnKeyUpRun}
              role="button"
              tabindex="0"
            />
          </div>
        {/if}

        <hr class="my-4 border-t border-medium" />

        <h2 class="font-bold">Live feeds</h2>

        <v-radio
          label="View"
          options="Stacked, Grid"
          selected={selectedView}
          disable={disableViews ? 'true' : 'false'}
          on:input={handleViewSelect}
        />

        {#if cameras}
          <div class="flex flex-col gap-2">
            {#each cameras as camera (camera.name)}
              <v-switch
                label={camera.name}
                aria-label={`Refresh frequency for ${camera.name}`}
                value={openCameras[camera.name] ? 'on' : 'off'}
                on:input={() => {
                  handleSwitch(camera.name);
                }}
              />
            {/each}

            <div class="mt-2 flex items-end gap-2">
              <v-select
                value={refreshFrequency}
                label="Refresh frequency"
                aria-label="Refresh frequency"
                options={Object.keys(selectedMap).join(',')}
                disabled={disableRefresh ? 'true' : 'false'}
                on:input={handleSetRefreshFrequency}
              />

              <v-button
                class={refreshFrequency === 'Live' ? 'invisible' : ''}
                icon="refresh"
                label="Refresh"
                disabled={disableRefresh ? 'true' : 'false'}
                on:click={() => {
                  triggerRefresh = !triggerRefresh;
                }}
                on:keyup ={() => {
                  triggerRefresh = !triggerRefresh;
                }}
                role="button"
                tabindex="0"
              />
            </div>
          </div>
        {/if}
      </div>
      <div
        class="justify-start gap-4 border-medium p-4 sm:border-l {selectedView ===
        'Stacked'
          ? 'flex flex-col'
          : 'grid grid-cols-2 gap-4'}"
      >
        <!-- ******* CAMERAS *******  -->
        {#each cameras as camera (`base ${camera.name}`)}
          {#if openCameras[camera.name]}
            <Camera
              cameraName={camera.name}
              showExportScreenshot
              refreshRate={refreshFrequency}
              triggerRefresh={triggerRefresh}
            />
          {/if}
        {/each}
      </div>
    </div>
  </Collapse>
</div>
