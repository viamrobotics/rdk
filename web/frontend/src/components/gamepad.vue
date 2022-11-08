<!-- eslint-disable id-length -->
<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { onMounted, onUnmounted, watch } from 'vue';
import { Timestamp } from 'google-protobuf/google/protobuf/timestamp_pb';
import InputController from '../gen/proto/api/component/inputcontroller/v1/input_controller_pb.esm';
import { displayError } from '../lib/error';

let gamepad = $ref<Gamepad | null>(null);
let gamepadConnected = $ref(false);
let gamepadConnectedPrev = $ref(false);
const enabled = $ref(false);

const curStates = $ref<Record<string, number>>({
  X: Number.NaN,
  Y: Number.NaN,
  RX: Number.NaN,
  RY: Number.NaN,
  Z: Number.NaN,
  RZ: Number.NaN,
  Hat0X: Number.NaN,
  Hat0Y: Number.NaN,
  South: Number.NaN,
  East: Number.NaN,
  West: Number.NaN,
  North: Number.NaN,
  LT: Number.NaN,
  RT: Number.NaN,
  LThumb: Number.NaN,
  RThumb: Number.NaN,
  Select: Number.NaN,
  Start: Number.NaN,
  Menu: Number.NaN,
});

let handle = -1;
let prevStates: Record<string, number> = {};

const sendEvent = (newEvent: InputController.Event) => {
  if (enabled) {
    const req = new InputController.TriggerEventRequest();
    req.setController('WebGamepad');
    req.setEvent(newEvent);
    window.inputControllerService.triggerEvent(req, new grpc.Metadata(), displayError);
  }
};

const connectEvent = (con: boolean) => {
  if (
    (con === true && gamepadConnected === false) ||
    (con === false && gamepadConnectedPrev === false)
  ) {
    return;
  }

  for (const ctrl of Object.keys(curStates)) {
    const newEvent = new InputController.Event();
    newEvent.setTime(Timestamp.fromDate(new Date()));
    newEvent.setEvent(con ? 'Connect' : 'Disconnect');
    newEvent.setValue(0);

    if ((/X|Y|Z$/u).test(ctrl)) {
      newEvent.setControl(`Absolute${ctrl}`);
    } else {
      newEvent.setControl(`Button${ctrl}`);
    }

    sendEvent(newEvent);
  }
};

const processEvents = () => {
  if (gamepadConnected === false) {
    for (const key of Object.keys(curStates)) {
      curStates[key] = Number.NaN;
    }

    if (gamepadConnectedPrev === true) {
      connectEvent(false);
      gamepadConnectedPrev = false;
    }
    return;
  } else if (gamepadConnectedPrev === false) {
    connectEvent(true);
    gamepadConnectedPrev = true;
  }

  for (const [key, value] of Object.entries(curStates)) {
    if (
      value === prevStates[key] ||
      (Number.isNaN(value) &&
        Number.isNaN(prevStates[key]))
    ) {
      continue;
    }
    const newEvent = new InputController.Event();
    newEvent.setTime(Timestamp.fromDate(new Date()));
    if ((/X|Y|Z$/u).test(key)) {
      newEvent.setControl(`Absolute${key}`);
      newEvent.setEvent('PositionChangeAbs');
    } else {
      newEvent.setControl(`Button${key}`);
      newEvent.setEvent(
        value ? 'ButtonPress' : 'ButtonRelease'
      );
    }

    if (Number.isNaN(value)) {
      newEvent.setEvent('Disconnect');
      newEvent.setValue(0);
    } else {
      newEvent.setValue(value);
    }
    sendEvent(newEvent);
  }
};

const tick = () => {
  let gamepadFound = false;
  const pads = navigator.getGamepads();
  for (const pad of pads) {
    if (pad) {
      gamepad = pad;
      gamepadFound = true;
      break;
    }
  }

  if (gamepadFound === false) {
    gamepadConnected = false;
    gamepad = null;
  }

  if (gamepad) {
    prevStates = { ...prevStates, ...curStates };
    gamepadConnected = gamepad.connected;

    curStates.X = gamepad.axes[0]!;
    curStates.Y = gamepad.axes[1]!;
    curStates.RX = gamepad.axes[2]!;
    curStates.RY = gamepad.axes[3]!;
    curStates.Z = gamepad.buttons[6]!.value;
    curStates.RZ = gamepad.buttons[7]!.value;
    curStates.Hat0X = (gamepad.buttons[14]!.value * -1) + gamepad.buttons[15]!.value;
    curStates.Hat0Y = (gamepad.buttons[12]!.value * -1) + gamepad.buttons[13]!.value;
    curStates.South = gamepad.buttons[0]!.value;
    curStates.East = gamepad.buttons[1]!.value;
    curStates.West = gamepad.buttons[2]!.value;
    curStates.North = gamepad.buttons[3]!.value;
    curStates.LT = gamepad.buttons[4]!.value;
    curStates.RT = gamepad.buttons[5]!.value;
    curStates.Select = gamepad.buttons[8]!.value;
    curStates.Start = gamepad.buttons[9]!.value;
    curStates.LThumb = gamepad.buttons[10]!.value;
    curStates.RThumb = gamepad.buttons[11]!.value;
    curStates.Menu = gamepad.buttons[16]!.value;
  }

  processEvents();
  handle = window.requestAnimationFrame(tick);
};

onMounted(() => {
  prevStates = { ...prevStates, ...curStates };
  tick();
});

onUnmounted(() => {
  cancelAnimationFrame(handle);
});

watch(() => enabled, () => {
  connectEvent(enabled);
});

</script>

<template>
  <v-collapse
    title="WebGamepad"
    class="do"
  >
    <v-breadcrumbs
      slot="title"
      crumbs="input_controller"
    />
    <div slot="header">
      <span
        v-if="gamepadConnected && enabled"
        class="rounded-full bg-green-500 px-3 py-0.5 text-xs text-white"
      >Connected</span>
      <span
        v-else
        class="rounded-full bg-gray-200 px-3 py-0.5 text-xs text-gray-800"
      >Disconnected</span>
    </div>

    <div class="h-full w-full border border-t-0 border-black p-4">
      <div class="flex flex-row">
        <label class="subtitle mr-2">Enabled</label>
        <v-switch
          :value="enabled ? 'on' : 'off'"
          @input="enabled = !enabled"
        />
      </div>

      <div
        v-if="gamepadConnected"
        class="flex h-full w-full flex-row justify-between gap-2"
      >
        <div
          v-for="(value, name) in curStates"
          :key="name"
          class="ml-0 flex w-[8ex] flex-col text-center"
        >
          <p class="subtitle m-0">
            {{ name }}
          </p>
          {{ /X|Y|Z$/.test(name.toString()) ? value!.toFixed(4) : value!.toFixed() }}
        </div>
      </div>
    </div>
  </v-collapse>
</template>

<style scoped>

.subtitle {
  color: var(--black-70);
}

</style>
