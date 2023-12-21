<script lang='ts'>
import { useGamepad } from '@threlte/extras';
import { Timestamp } from 'google-protobuf/google/protobuf/timestamp_pb';
import { ConnectionClosedError } from '@viamrobotics/rpc';
import { inputControllerApi as InputController, type ServiceError } from '@viamrobotics/sdk';
import { notify } from '@viamrobotics/prime';
import { rcLogConditionally } from '@/lib/log';
import Collapse from '@/lib/components/collapse.svelte';
import { useRobotClient } from '@/hooks/robot-client';

export let name: string;

const { robotClient } = useRobotClient();
const gamepad = useGamepad();

let enabled = false;

const gamepadKeys = [
  'X',
  'Y',
  'RX',
  'RY',
  'Z',
  'RZ',
  'Hat0X',
  'Hat0Y',
  'South',
  'East',
  'West',
  'North',
  'LT',
  'RT',
  'LThumb',
  'RThumb',
  'Select',
  'Start',
  'Menu',
] as const;

let lastError = Date.now();

const round = (num: number, decimals = 4): number => {
  return Math.round(num * 10 ** decimals) / 10 ** decimals;
}

const isAxis = (key: string) => {
  return ['X', 'Y', 'RX', 'RY'].includes(key)
}

const sendEvent = (newEvent: InputController.Event) => {
  if (!enabled) {
    return;
  }
  const req = new InputController.TriggerEventRequest();
  req.setController(name);
  req.setEvent(newEvent);
  rcLogConditionally(req);
  $robotClient.inputControllerService.triggerEvent(req, (error: ServiceError | null) => {
    if (error) {
      if (ConnectionClosedError.isError(error)) {
        return;
      }
      const now = Date.now();
      if (now - lastError > 1000) {
        lastError = now;
        notify.danger(error.message);
      }
    }
  });
};

let lastTS = Timestamp.fromDate(new Date());

const nextTS = () => {
  let nowTS = Timestamp.fromDate(new Date());
  if (lastTS.getSeconds() > nowTS.getSeconds() ||
    (lastTS.getSeconds() === nowTS.getSeconds() && lastTS.getNanos() > nowTS.getNanos())) {
    nowTS = lastTS;
  }
  if (nowTS.getSeconds() === lastTS.getSeconds() &&
    nowTS.getNanos() === lastTS.getNanos()) {
    nowTS.setNanos(nowTS.getNanos() + 1);
  }
  lastTS = nowTS;
  return nowTS;
};

const updateConnection = (connected: boolean) => {
  const nowTS = nextTS();
  for (const key of gamepadKeys) {
    const event = new InputController.Event();
    nowTS.setNanos(nowTS.getNanos() + 1);
    event.setTime(nowTS);
    event.setEvent(connected ? 'Connect' : 'Disconnect');
    event.setValue(0);
    event.setControl(isAxis(key) ? `Absolute${key}` : `Button${key}`);
    sendEvent(event);
    lastTS = nowTS;
  }
};

const process = (key: typeof gamepadKeys[number], value: number) => {
  const nowTS = nextTS();
  const event = new InputController.Event();
  nowTS.setNanos(nowTS.getNanos() + 1);
  event.setTime(nowTS);
  event.setControl(isAxis(key) ? `Absolute${key}` : `Button${key}`);
  event.setEvent(isAxis(key) ? 'PositionChangeAbs' : `Button${value === 0 ? 'Release' : 'Press'}`);
  event.setValue(value === 0 ? 0 : round(value));
  sendEvent(event);
  lastTS = nowTS;
};

gamepad.leftStick.on('change', ({ value }) => process('X', value.x))
gamepad.leftStick.on('change', ({ value }) => process('Y', value.y))
gamepad.rightStick.on('change', ({ value }) => process('RX', value.x))
gamepad.rightStick.on('change', ({ value }) => process('RY', value.y))

gamepad.leftTrigger.on('change', ({ value }) => process('Z', value))
gamepad.rightTrigger.on('change', ({ value }) => process('RZ', value))

const onDirXChange = () => process('Hat0X', -gamepad.directionalLeft.value + gamepad.directionalRight.value)
gamepad.directionalLeft.on('change', onDirXChange)
gamepad.directionalRight.on('change', onDirXChange)

const onDirYChange = () => process('Hat0Y', -gamepad.directionalTop.value + gamepad.directionalBottom.value)
gamepad.directionalTop.on('change', onDirYChange)
gamepad.directionalBottom.on('change', onDirYChange)

gamepad.clusterBottom.on('change', ({ value }) => process('South', value))
gamepad.clusterRight.on('change', ({ value }) => process('East', value))
gamepad.clusterLeft.on('change', ({ value }) => process('West', value))

  curStates.West = trunc(gamepad.buttons[2]?.value);
  curStates.North = trunc(gamepad.buttons[3]?.value);
  curStates.LT = trunc(gamepad.buttons[4]?.value);
  curStates.RT = trunc(gamepad.buttons[5]?.value);
  curStates.Select = trunc(gamepad.buttons[8]?.value);
  curStates.Start = trunc(gamepad.buttons[9]?.value);
  curStates.LThumb = trunc(gamepad.buttons[10]?.value);
  curStates.RThumb = trunc(gamepad.buttons[11]?.value);
  curStates.Menu = trunc(gamepad.buttons[16]?.value);

$: connected = gamepad.connected
$: updateConnection($connected && enabled);

</script>

<Collapse title={name}>
  <svelte:fragment slot='title'>
    <v-breadcrumbs crumbs="input_controller" />

    {#if $connected}
      ({gamepad.raw?.id})
    {/if}
  </svelte:fragment>

  <div slot="header">
    {#if $connected && enabled}
      <v-badge variant='green' label='Enabled' />
    {:else}
      <v-badge variant='gray' label='Disabled' />
    {/if}
  </div>

  <div class="h-full w-full border border-t-0 border-medium p-4">
    <div class="flex flex-row">
      <v-switch
        label='Enable gamepad'
        value={enabled ? 'on' : 'off'}
        on:input={() => (enabled = !enabled)}
      />
    </div>

    {#if $connected}
      <div class="flex h-full w-full flex-row justify-between gap-2">
        {#each Object.keys({}) as stateName, value}
          <div class="ml-0 flex w-[8ex] flex-col text-center">
            <p class="subtitle m-0">{stateName}</p>
            {value.toFixed((/X|Y|Z$/u).test(stateName.toString()) ? 4 : 0)}
          </div>
        {/each}
      </div>
    {/if}
  </div>
</Collapse>

<style>

.subtitle {
  color: var(--black-70);
}
</style>
