<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import {
    BaseClient,
    Client,
    type ServiceError,
    commonApi,
    type ResponseStream,
    robotApi,
  } from "@viamrobotics/sdk";
  import { filterResources } from "../../lib/resource";
  import { displayError } from "../../lib/error";
  import KeyboardInput, { type Keys } from "../keyboard-input/index.svelte";
  import Camera from "../camera/camera.svelte";
  import { rcLogConditionally } from "../../lib/log";
  import { selectedMap } from "../../lib/camera-state";
  import { clickOutside } from "../../lib/click-outside";
  import type { StreamManager } from "./camera/stream-manager";

  export let name: string;
  export let resources: commonApi.ResourceName.AsObject[];
  export let client: Client;
  export let streamManager: StreamManager;
  export let statusStream: ResponseStream<robotApi.StreamStatusResponse> | null;

  const enum Keymap {
    LEFT = "a",
    RIGHT = "d",
    FORWARD = "w",
    BACKWARD = "s",
  }

  type Tabs = "Keyboard" | "Discrete";
  type MovementTypes = "Continuous" | "Discrete";
  type MovementModes = "Straight" | "Spin";
  type SpinTypes = "Clockwise" | "Counterclockwise";
  type Directions = "Forwards" | "Backwards";
  type View = "Stacked" | "Grid";

  const baseClient = new BaseClient(client, name, {
    requestLogger: rcLogConditionally,
  });

  let refreshFrequency = "Live";
  let triggerRefresh = false;

  const openCameras: Record<string, boolean | undefined> = {};
  let selectedView: View = "Stacked";
  let selectedMode: Tabs = "Keyboard";
  let movementMode: MovementModes = "Straight";
  let movementType: MovementTypes = "Continuous";
  let direction: Directions = "Forwards";
  let spinType: SpinTypes = "Clockwise";
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

  $: filteredResources = filterResources(
    resources,
    "rdk",
    "component",
    "camera"
  );

  const resetDiscreteState = () => {
    movementMode = "Straight";
    movementType = "Continuous";
    direction = "Forwards";
    spinType = "Clockwise";
  };

  const setMovementMode = (mode: MovementModes) => {
    movementMode = mode;
  };

  const setMovementType = (type: MovementTypes) => {
    movementType = type;
  };

  const setSpinType = (type: SpinTypes) => {
    spinType = type;
  };

  const setDirection = (dir: Directions) => {
    direction = dir;
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
    if (event.movementType === "Continuous") {
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
    if (movementMode === "Spin") {
      try {
        await baseClient.spin(
          angle * (spinType === "Clockwise" ? -1 : 1),
          spinSpeed
        );
      } catch (error) {
        displayError(error as ServiceError);
      }
    } else if (movementMode === "Straight") {
      handleBaseStraight({
        movementType,
        direction: direction === "Forwards" ? 1 : -1,
        speed,
        distance: increment,
      });
    }
  };

  const handleViewSelect = (viewMode: View) => {
    selectedView = viewMode;

    let liveCameras = 0;
    for (const camera of filteredResources) {
      if (openCameras[camera.name]) {
        liveCameras += 1;
      }
    }
    disableViews = liveCameras > 1;
  };

  const handleTabSelect = (controlMode: Tabs) => {
    selectedMode = controlMode;

    if (controlMode === "Discrete") {
      resetDiscreteState();
    }
  };

  const handleVisibilityChange = () => {
    // only issue the stop if there are keys pressed as to not interfere with commands issued by scripts/commands
    if (document.visibilityState === "hidden" && pressed.size > 0) {
      pressed.clear();
      stop();
    }
  };

  const handleOnBlur = () => {
    if (pressed.size <= 0) {
      stop();
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

    for (const camera of filteredResources) {
      if (openCameras[camera.name]) {
        disableRefresh = false;
        return;
      }
    }
    disableRefresh = true;
  };

  onMount(() => {
    window.addEventListener("visibilitychange", handleVisibilityChange);

    // Safety measure for system prompts, etc.
    window.addEventListener("blur", handleOnBlur);

    for (const camera of filteredResources) {
      openCameras[camera.name] = false;
    }
  });

  onDestroy(() => {
    handleOnBlur();

    window.removeEventListener("visibilitychange", handleVisibilityChange);
    window.removeEventListener("blur", handleOnBlur);
  });
</script>

<div
  use:clickOutside={() => {
    isKeyboardActive = false;
  }}
>
  <v-collapse title={name} class="base">
    <v-breadcrumbs slot="title" crumbs="base" />

    <v-button
      slot="header"
      variant="danger"
      icon="stop-circle"
      label="Stop"
      on:click={stop}
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
          on:input={(event) => {
            handleTabSelect(event.detail.value);
          }}
        />

        {#if selectedMode === "Keyboard"}
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
              value="power"
              on:input={(event) => {
                power = event.detail.value;
              }}
            />
          </div>
        {/if}

        {#if selectedMode === "Discrete"}
          <div class="flex flex-col gap-4">
            <v-radio
              label="Movement mode"
              options="Straight, Spin"
              selected={movementMode}
              on:input={(event) => {
                setMovementMode(event.detail.value);
              }}
            />
            {#if movementMode === "Straight"}
              <v-radio
                label="Movement type"
                options="Continuous, Discrete"
                selected={movementType}
                on:input={(event) => {
                  setMovementType(event.detail.value);
                }}
              />
              <v-radio
                label="Direction"
                options="Forwards, Backwards"
                selected={direction}
                on:input={(event) => {
                  setDirection(event.detail.value);
                }}
              />
              <v-input
                type="number"
                value={speed}
                label="Speed (mm/sec)"
                on:input={(event) => {
                  speed = event.detail.value;
                }}
              />
              <div
                class="pointer-events-none"
                class:opacity-50={movementType === "Continuous"}
              >
                <v-input
                  type="number"
                  value={increment}
                  readonly={movementType === "Continuous" ? true : false}
                  label="Distance (mm)"
                  on:input={(event) => {
                    increment = event.detail.value;
                  }}
                />
              </div>
            {/if}
            {#if movementMode === "Spin"}
              <v-input
                type="number"
                value={spinSpeed}
                label="Speed (deg/sec)"
                on:input={(event) => {
                  spinSpeed = event.detail.value;
                }}
              />
              <v-radio
                label="Movement Type"
                options="Clockwise, Counterclockwise"
                selected={spinType}
                on:input={(event) => {
                  setSpinType(event.detail.value);
                }}
              />
              <div>
                <v-slider
                  min={0}
                  max={360}
                  step={15}
                  suffix="°"
                  label="Angle"
                  value={angle}
                  on:input={(event) => {
                    angle = event.detail.value;
                  }}
                />
              </div>
              <v-button
                icon="play-circle-filled"
                variant="success"
                label="Run"
                on:click={baseRun}
              />
            {/if}
          </div>
        {/if}

        <hr class="my-4 border-t border-medium" />

        <h2 class="font-bold">Live feeds</h2>

        <v-radio
          label="View"
          options="Stacked, Grid"
          selected={selectedView}
          disable={disableViews ? "true" : "false"}
          on:input={(event) => {
            handleViewSelect(event.detail.value);
          }}
        />

        {#if filteredResources}
          <div class="flex flex-col gap-2">
            {#each filteredResources as camera (camera.name)}
              <v-switch
                label={camera.name}
                aria-label={`Refresh frequency for ${camera.name}`}
                value={openCameras[camera.name] ? "on" : "off"}
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
                options={Object.keys(selectedMap).join(",")}
                disabled={disableRefresh ? "true" : "false"}
                on:input={(event) => {
                  refreshFrequency = event.detail.value;
                }}
              />

              <v-button
                class={refreshFrequency === "Live" ? "invisible" : ""}
                icon="refresh"
                label="Refresh"
                disabled={disableRefresh ? "true" : "false"}
                on:click={() => {
                  triggerRefresh = !triggerRefresh;
                }}
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
        {#each filteredResources as camera (`base ${camera.name}`)}
          {#if openCameras[camera.name]}
            <Camera
              cameraName={camera.name}
              {client}
              showExportScreenshot
              refreshRate={refreshFrequency}
              {streamManager}
              {statusStream}
            />
          {/if}
        {/each}
      </div>
    </div>
  </v-collapse>
</div>
