<script lang="ts">
import { useRobotClient } from '@/hooks/robot-client';
import Client from '@/lib/components/robot-client.svelte';
import {
  filterSubtype,
  filterWithStatus,
  resourceNameToString,
} from '@/lib/resource';
import type { RCOverrides } from '@/types/overrides';
import { ResourceName, type Credential } from '@viamrobotics/sdk';
import Arm from './arm/index.svelte';
import AudioInput from './audio-input/index.svelte';
import Base from './base/index.svelte';
import Board from './board/index.svelte';
import CamerasList from './camera/index.svelte';
import DoCommand from './do-command/index.svelte';
import Encoder from './encoder/index.svelte';
import Gamepad from './gamepad/index.svelte';
import Gantry from './gantry/index.svelte';
import Gripper from './gripper/index.svelte';
import InputController from './input-controller/index.svelte';
import Motor from './motor/index.svelte';
import MovementSensor from './movement-sensor/index.svelte';
import Navigation from './navigation/index.svelte';
import OperationsSessions from './operations-sessions/index.svelte';
import PowerSensor from './power-sensor/index.svelte';
import Sensors from './sensors/index.svelte';
import Servo from './servo/index.svelte';
import Slam from './slam/index.svelte';
import Vision from './vision/index.svelte';

const { resources, components, services, statuses, sensorNames } =
  useRobotClient();

export let host: string;
export let bakedAuth:
  | {
      authEntity?: string;
      creds?: Credential;
    }
  | undefined = {};
export let supportedAuthTypes: string[] | undefined = [];
export let webrtcEnabled: boolean;
export let signalingAddress: string;
export let overrides: RCOverrides | undefined = undefined;
/* Used in the control tab project to incrementally deliver and conditionally disable showing API subtypes. */
export let hiddenSubtypes: string[] = [];
export let hideDoCommand = false;
export let hideOperationsSessions = false;

$: hidden = new Set(hiddenSubtypes);

const resourceStatusByName = (resource: ResourceName) => {
  return $statuses[resourceNameToString(resource)];
};

// TODO (APP-146): replace these with constants
$: filteredWebGamepads = $components.filter((component) => {
  const remSplit = component.name.split(':');
  return (
    component.subtype === 'input_controller' &&
    Boolean(component.name) &&
    remSplit.at(-1) === 'WebGamepad'
  );
});

/*
 * TODO (APP-146): replace these with constants
 * filters out WebGamepad
 */
$: filteredInputControllerList = $components.filter((component) => {
  const remSplit = component.name.split(':');
  return (
    component.subtype === 'input_controller' &&
    Boolean(component.name) &&
    remSplit.at(-1) !== 'WebGamepad' &&
    resourceStatusByName(component)
  );
});

const getStatus = (
  statusMap: Record<string, unknown>,
  resource: ResourceName
) => {
  const key = resourceNameToString(resource);
  // todo(mp) Find a way to fix this type error
  // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-return
  return key ? (statusMap[key] as any) : undefined;
};
</script>

<Client
  {webrtcEnabled}
  {host}
  {signalingAddress}
  {bakedAuth}
  {supportedAuthTypes}
>
  <v-notify
    slot="connecting"
    class="p-3"
    variant="info"
    title={`Connecting via ${webrtcEnabled ? 'WebRTC' : 'gRPC'}...`}
  />

  <v-notify
    slot="reconnecting"
    variant="danger"
    title="Connection lost, attempting to reconnect ..."
  />

  <div class="flex flex-col gap-4 p-3">
    <!-- ******* BASE *******  -->
    {#if !hidden.has('base')}
      {#each filterSubtype($components, 'base') as { name } (name)}
        <Base {name} />
      {/each}
    {/if}

    <!-- ******* SLAM *******  -->
    {#if !hidden.has('slam')}
      {#each filterSubtype($services, 'slam') as { name } (name)}
        <Slam
          {name}
          motionResourceNames={filterSubtype($services, 'motion')}
          overrides={overrides?.slam}
        />
      {/each}
    {/if}

    <!-- ******* ENCODER *******  -->
    {#if !hidden.has('encoder')}
      {#each filterSubtype($components, 'encoder') as { name } (name)}
        <Encoder {name} />
      {/each}
    {/if}

    <!-- ******* GANTRY *******  -->
    {#if !hidden.has('gantry')}
      {#each filterWithStatus($components, $statuses, 'gantry') as gantry (gantry.name)}
        <Gantry
          name={gantry.name}
          status={getStatus($statuses, gantry)}
        />
      {/each}
    {/if}

    <!-- ******* MOVEMENT SENSOR *******  -->
    {#if !hidden.has('movement_sensor')}
      {#each filterSubtype($components, 'movement_sensor') as { name } (name)}
        <MovementSensor {name} />
      {/each}
    {/if}

    <!-- ******* POWER SENSOR *******  -->
    {#if !hidden.has('power_sensor')}
      {#each filterSubtype($components, 'power_sensor') as { name } (name)}
        <PowerSensor {name} />
      {/each}
    {/if}

    <!-- ******* ARM *******  -->
    {#if !hidden.has('arm')}
      {#each filterSubtype($components, 'arm') as arm (arm.name)}
        <Arm
          name={arm.name}
          status={getStatus($statuses, arm)}
        />
      {/each}
    {/if}

    <!-- ******* GRIPPER *******  -->
    {#if !hidden.has('gripper')}
      {#each filterSubtype($components, 'gripper') as { name } (name)}
        <Gripper {name} />
      {/each}
    {/if}

    <!-- ******* SERVO *******  -->
    {#if !hidden.has('servo')}
      {#each filterWithStatus($components, $statuses, 'servo') as servo (servo.name)}
        <Servo
          name={servo.name}
          status={getStatus($statuses, servo)}
        />
      {/each}
    {/if}

    <!-- ******* MOTOR *******  -->
    {#if !hidden.has('motor')}
      {#each filterWithStatus($components, $statuses, 'motor') as motor (motor.name)}
        <Motor
          name={motor.name}
          status={getStatus($statuses, motor)}
        />
      {/each}
    {/if}

    <!-- ******* INPUT VIEW *******  -->
    {#if !hidden.has('input_controller')}
      {#each filteredInputControllerList as controller (controller.name)}
        <InputController
          name={controller.name}
          status={getStatus($statuses, controller)}
        />
      {/each}
    {/if}

    <!-- ******* WEB CONTROLS *******  -->
    {#if !hidden.has('input_controller')}
      {#each filteredWebGamepads as { name } (name)}
        <Gamepad {name} />
      {/each}
    {/if}

    <!-- ******* BOARD *******  -->
    {#if !hidden.has('board')}
      {#each filterWithStatus($components, $statuses, 'board') as board (board.name)}
        <Board
          name={board.name}
          status={getStatus($statuses, board)}
        />
      {/each}
    {/if}

    <!-- ******* CAMERA *******  -->
    {#if !hidden.has('camera')}
      <CamerasList resources={filterSubtype($components, 'camera')} />
    {/if}

    <!-- ******* NAVIGATION *******  -->
    {#if !hidden.has('navigation')}
      {#each filterSubtype($services, 'navigation') as { name } (name)}
        <Navigation {name} />
      {/each}
    {/if}

    <!-- ******* SENSOR *******  -->
    {#if !hidden.has('sensors')}
      {#if Object.keys($sensorNames).length > 0}
        <Sensors
          name={filterSubtype($resources, 'sensors', { remote: false })[0]
            ?.name ?? ''}
          sensorNames={$sensorNames}
        />
      {/if}
    {/if}

    <!-- ******* AUDIO *******  -->
    {#if !hidden.has('audio_input')}
      {#each filterSubtype($components, 'audio_input') as { name } (name)}
        <AudioInput {name} />
      {/each}
    {/if}

    <!-- ******* VISION *******  -->
    {#if !hidden.has('vision')}
      {#if filterSubtype($services, 'vision').length > 0}
        <Vision names={filterSubtype($services, 'vision').map((v) => v.name)} />
      {/if}
    {/if}

    <!-- ******* DO *******  -->
    {#if !hideDoCommand}
      <DoCommand resources={[...$components, ...$services]} />
    {/if}

    <!-- ******* OPERATIONS AND SESSIONS *******  -->
    {#if !hideOperationsSessions}
      <OperationsSessions />
    {/if}
  </div>
</Client>
