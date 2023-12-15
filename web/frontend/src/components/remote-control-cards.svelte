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
import Collapse from '@/lib/components/collapse.svelte';
import type { RCOverrides } from '@/types/overrides';

const { resources, components, services, statuses, sensorNames, sessionsSupported } = useRobotClient();

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
    remSplit.at(-1) !== 'WebGamepad' && resourceStatusByName(component)
  );
});

const getStatus = (statusMap: Record<string, unknown>, resource: commonApi.ResourceName.AsObject) => {
  const key = resourceNameToString(resource);
  // todo(mp) Find a way to fix this type error
  // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-return
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
      <Collapse title={name} crumbs='base' hasStop let:onStop>
        <Base {name} {onStop} />
      </Collapse>
    {/each}

    <!-- ******* SLAM *******  -->
    {#each filterSubtype($services, 'slam') as { name } (name)}
      <Collapse title={name} crumbs="slam" hasStop let:onStop>
        <Slam {name} overrides={overrides?.slam} {onStop} />
      </Collapse>
    {/each}

    <!-- ******* ENCODER *******  -->
    {#each filterSubtype($components, 'encoder') as { name } (name)}
      <Collapse title={name} crumbs="encoder">
        <Encoder {name} />
      </Collapse>
    {/each}

    <!-- ******* GANTRY *******  -->
    {#each filterWithStatus($components, $statuses, 'gantry') as gantry (gantry.name)}
      <Collapse title={gantry.name} crumbs="gantry" hasStop let:onStop>
        <Gantry
          name={gantry.name}
          status={getStatus($statuses, gantry)}
          {onStop}
        />
      </Collapse>
    {/each}

    <!-- ******* MOVEMENT SENSOR *******  -->
    {#each filterSubtype($components, 'movement_sensor') as { name } (name)}
      <Collapse title={name} crumbs="movement_sensor">
        <MovementSensor {name} />
      </Collapse>
    {/each}

     <!-- ******* POWER SENSOR *******  -->
    {#each filterSubtype($components, 'power_sensor') as { name } (name)}
      <Collapse title={name} crumbs="power_sensor">
        <PowerSensor {name} />
      </Collapse>
    {/each}

    <!-- ******* ARM *******  -->
    {#each filterSubtype($components, 'arm') as arm (arm.name)}
      <Collapse title={arm.name} crumbs="arm" hasStop let:onStop>
        <Arm
          name={arm.name}
          status={getStatus($statuses, arm)}
          {onStop}
        />
      </Collapse>
    {/each}

    <!-- ******* GRIPPER *******  -->
    {#each filterSubtype($components, 'gripper') as { name } (name)}
      <Collapse title={name} crumbs="gripper" hasStop let:onStop>
        <Gripper {name} {onStop} />
      </Collapse>
    {/each}

    <!-- ******* SERVO *******  -->
    {#each filterWithStatus($components, $statuses, 'servo') as servo (servo.name)}
      <Collapse title={servo.name} crumbs="servo" hasStop let:onStop>
        <Servo
          name={servo.name}
          status={getStatus($statuses, servo)}
          {onStop}
        />
      </Collapse>
    {/each}

    <!-- ******* MOTOR *******  -->
    {#each filterWithStatus($components, $statuses, 'motor') as motor (motor.name)}
      <Collapse title={motor.name} crumbs="motor" hasStop let:onStop>
        <Motor
          name={motor.name}
          status={getStatus($statuses, motor)}
          {onStop}
        />
      </Collapse>
    {/each}

    <!-- ******* INPUT VIEW *******  -->
    {#each filteredInputControllerList as controller (controller.name)}
      <Collapse title={controller.name} crumbs="input_controller">
        <InputController
          name={controller.name}
          status={getStatus($statuses, controller)}
        />
      </Collapse>
    {/each}

    <!-- ******* WEB CONTROLS *******  -->
    {#each filteredWebGamepads as { name } (name)}
      <Collapse title={name} crumbs='input_controller'>
        <Gamepad {name} />
      </Collapse>
    {/each}

    <!-- ******* BOARD *******  -->
    {#each filterWithStatus($components, $statuses, 'board') as board (board.name)}
      <Collapse title={board.name} crumbs="board">
        <Board
          name={board.name}
          status={getStatus($statuses, board)}
        />
      </Collapse>
    {/each}

    <!-- ******* CAMERA *******  -->
    <CamerasList resources={filterSubtype($components, 'camera')} />

    <!-- ******* NAVIGATION *******  -->
    {#each filterSubtype($services, 'navigation') as { name } (name)}
      <Collapse title={name} crumbs='navigation' hasStop let:onStop>
        <Navigation {name} {onStop} />
      </Collapse>
    {/each}

    <!-- ******* SENSOR *******  -->
    {#if Object.keys($sensorNames).length > 0}
      <Collapse title="Sensors">
        <Sensors
          name={filterSubtype($resources, 'sensors', { remote: false })[0]?.name ?? ''}
          sensorNames={$sensorNames}
        />
      </Collapse>
    {/if}

    <!-- ******* AUDIO *******  -->
    {#each filterSubtype($components, 'audio_input') as { name } (name)}
      <Collapse title={name} crumbs="audio_input">
        <AudioInput {name} />
      </Collapse>
    {/each}

    <!-- ******* DO *******  -->
    {#if filterSubtype($components, 'generic').length > 0}
      <Collapse title="DoCommand()">
        <DoCommand resources={filterSubtype($components, 'generic')} />
      </Collapse>
    {/if}

    <!-- ******* OPERATIONS AND SESSIONS *******  -->
    <Collapse title={$sessionsSupported ? 'Operations & Sessions' : 'Operations'}>
      <OperationsSessions />
    </Collapse>
  </div>
</Client>
