<script lang="ts">

import { type Credentials } from '@viamrobotics/rpc';
import { commonApi } from '@viamrobotics/sdk';
import { resourceNameToString, filterWithStatus, filterSubtype } from '@/lib/resource';
import { useRobotClient } from '@/hooks/robot-client';
import Arm from './arm/index.svelte';
import AudioInput from './audio-input/index.svelte';
import Base from './base/index.svelte';
import Board from './board/index.svelte';
import CamerasList from './camera/index.svelte';
import OperationsSessions from './operations-sessions/index.svelte';
import DoCommand from './do-command/index.svelte';
import Encoder from './encoder/index.svelte';
import Gantry from './gantry/index.svelte';
import Gripper from './gripper/index.svelte';
import Gamepad from './gamepad/index.svelte';
import InputController from './input-controller/index.svelte';
import Motor from './motor/index.svelte';
import MovementSensor from './movement-sensor/index.svelte';
import Navigation from './navigation/index.svelte';
import PowerSensor from './power-sensor/index.svelte';
import Servo from './servo/index.svelte';
import Sensors from './sensors/index.svelte';
import Slam from './slam/index.svelte';
import Client from '@/lib/components/robot-client.svelte';
import type { RCOverrides } from '@/types/overrides';

const { resources, components, services, statuses, sensorNames } = useRobotClient();

export let host: string;
export let bakedAuth: { authEntity?: string; creds?: Credentials; } | undefined = {};
export let supportedAuthTypes: string[] | undefined = [];
export let webrtcEnabled: boolean;
export let signalingAddress: string;
export let overrides: RCOverrides | undefined = undefined;

const resourceStatusByName = (resource: commonApi.ResourceName.AsObject) => {
  return $statuses[resourceNameToString(resource)];
};

// TODO (APP-146): replace these with constants
$: filteredWebGamepads = $components.filter((component) => {
  const remSplit = component.name.split(':');
  return (
    component.subtype === 'input_controller' &&
    Boolean(component.name) &&
    remSplit[remSplit.length - 1] === 'WebGamepad'
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
    remSplit[remSplit.length - 1] !== 'WebGamepad' && resourceStatusByName(component)
  );
});

const getStatus = (statusMap: Record<string, unknown>, resource: commonApi.ResourceName.AsObject) => {
  const key = resourceNameToString(resource);
  // todo(mp) Find a way to fix this type error
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  return key ? statusMap[key] as any : undefined;
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
    slot='connecting'
    class='p-3'
    variant='info'
    title={`Connecting via ${webrtcEnabled ? 'WebRTC' : 'gRPC'}...`}
  />

  <v-notify
    slot='reconnecting'
    variant='danger'
    title='Connection lost, attempting to reconnect ...'
  />

  <div class="flex flex-col gap-4 p-3">
    <!-- ******* BASE *******  -->
    {#each filterSubtype($components, 'base') as { name } (name)}
      <Base {name} />
    {/each}

    <!-- ******* SLAM *******  -->
    {#each filterSubtype($services, 'slam') as { name } (name)}
      <Slam {name} overrides={overrides?.slam} />
    {/each}

    <!-- ******* ENCODER *******  -->
    {#each filterSubtype($components, 'encoder') as { name } (name)}
      <Encoder {name} />
    {/each}

    <!-- ******* GANTRY *******  -->
    {#each filterWithStatus($components, $statuses, 'gantry') as gantry (gantry.name)}
      <Gantry
        name={gantry.name}
        status={getStatus($statuses, gantry)}
      />
    {/each}

    <!-- ******* MOVEMENT SENSOR *******  -->
    {#each filterSubtype($components, 'movement_sensor') as { name } (name)}
      <MovementSensor {name} />
    {/each}

     <!-- ******* POWER SENSOR *******  -->
    {#each filterSubtype($components, 'power_sensor') as { name } (name)}
      <PowerSensor {name} />
    {/each}

    <!-- ******* ARM *******  -->
    {#each filterSubtype($components, 'arm') as arm (arm.name)}
      <Arm
        name={arm.name}
        status={getStatus($statuses, arm)}
      />
    {/each}

    <!-- ******* GRIPPER *******  -->
    {#each filterSubtype($components, 'gripper') as { name } (name)}
      <Gripper {name} />
    {/each}

    <!-- ******* SERVO *******  -->
    {#each filterWithStatus($components, $statuses, 'servo') as servo (servo.name)}
      <Servo
        name={servo.name}
        status={getStatus($statuses, servo)}
      />
    {/each}

    <!-- ******* MOTOR *******  -->
    {#each filterWithStatus($components, $statuses, 'motor') as motor (motor.name)}
      <Motor
        name={motor.name}
        status={getStatus($statuses, motor)}
      />
    {/each}

    <!-- ******* INPUT VIEW *******  -->
    {#each filteredInputControllerList as controller (controller.name)}
      <InputController
        name={controller.name}
        status={getStatus($statuses, controller)}
      />
    {/each}

    <!-- ******* WEB CONTROLS *******  -->
    {#each filteredWebGamepads as { name } (name)}
      <Gamepad {name} />
    {/each}

    <!-- ******* BOARD *******  -->
    {#each filterWithStatus($components, $statuses, 'board') as board (board.name)}
      <Board
        name={board.name}
        status={getStatus($statuses, board)}
      />
    {/each}

    <!-- ******* CAMERA *******  -->
    <CamerasList resources={filterSubtype($components, 'camera')} />

    <!-- ******* NAVIGATION *******  -->
    {#each filterSubtype($services, 'navigation') as { name } (name)}
      <Navigation {name} />
    {/each}

    <!-- ******* SENSOR *******  -->
    {#if Object.keys($sensorNames).length > 0}
      <Sensors
        name={filterSubtype($resources, 'sensors', { remote: false })[0]?.name ?? ''}
        sensorNames={$sensorNames}
      />
    {/if}

    <!-- ******* AUDIO *******  -->
    {#each filterSubtype($components, 'audio_input') as { name } (name)}
      <AudioInput {name} />
    {/each}

    <!-- ******* DO *******  -->
    {#if filterSubtype($components, 'generic').length > 0}
      <DoCommand resources={filterSubtype($components, 'generic')} />
    {/if}

    <!-- ******* OPERATIONS AND SESSIONS *******  -->
    <OperationsSessions />
  </div>
</Client>
