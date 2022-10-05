<!-- eslint-disable id-length -->
<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { ref, onMounted, onUnmounted, watch } from 'vue';
import { Timestamp } from 'google-protobuf/google/protobuf/timestamp_pb';
import InputController from '../gen/proto/api/component/inputcontroller/v1/input_controller_pb.esm';
import { displayError } from '../lib/error';

const gamepad = ref(navigator.getGamepads()[0]);
const gamepadName = ref('Waiting for gamepad...');
const gamepadConnected = ref(false);
const gamepadConnectedPrev = ref(false);
const enabled = ref(false);

const curStates: Record<string, number> = {
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
};

let handle = -1;
let prevStates: Record<string, number> = {};

const sendEvent = (newEvent: InputController.Event) => {
  if (enabled.value) {
    const req = new InputController.TriggerEventRequest();
    req.setController('WebGamepad');
    req.setEvent(newEvent);
    window.inputControllerService.triggerEvent(req, new grpc.Metadata(), displayError);
  }
};

const connectEvent = (con: boolean) => {
  if (
    (con === true && gamepadConnected.value === false) ||
    (con === false && gamepadConnectedPrev.value === false)
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
  if (gamepadConnected.value === false) {
    for (const key of Object.keys(curStates)) {
      curStates[key] = Number.NaN;
    }

    if (gamepadConnectedPrev.value === true) {
      connectEvent(false);
      gamepadConnectedPrev.value = false;
    }
    return;
  } else if (gamepadConnectedPrev.value === false) {
    connectEvent(true);
    gamepadConnectedPrev.value = true;
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
      gamepad.value = pad;
      gamepadFound = true;
      break;
    }
  }

  if (gamepadFound === false) {
    gamepadName.value = 'Waiting for gamepad...';
    gamepadConnected.value = false;
    gamepad.value = null;
  }

  if (gamepad.value) {
    gamepadName.value = gamepad.value.mapping === 'standard'
      ? gamepad.value.id.replace(/ \(standard .*\)/iu, '')
      : gamepad.value.id;

    prevStates = { ...prevStates, ...curStates };
    gamepadConnected.value = gamepad.value.connected;

    curStates.X = gamepad.value.axes[0]!;
    curStates.Y = gamepad.value.axes[1]!;
    curStates.RX = gamepad.value.axes[2]!;
    curStates.RY = gamepad.value.axes[3]!;
    curStates.Z = gamepad.value.buttons[6]!.value;
    curStates.RZ = gamepad.value.buttons[7]!.value;
    curStates.Hat0X = (gamepad.value.buttons[14]!.value * -1) + gamepad.value.buttons[15]!.value;
    curStates.Hat0Y = (gamepad.value.buttons[12]!.value * -1) + gamepad.value.buttons[13]!.value;
    curStates.South = gamepad.value.buttons[0]!.value;
    curStates.East = gamepad.value.buttons[1]!.value;
    curStates.West = gamepad.value.buttons[2]!.value;
    curStates.North = gamepad.value.buttons[3]!.value;
    curStates.LT = gamepad.value.buttons[4]!.value;
    curStates.RT = gamepad.value.buttons[5]!.value;
    curStates.Select = gamepad.value.buttons[8]!.value;
    curStates.Start = gamepad.value.buttons[9]!.value;
    curStates.LThumb = gamepad.value.buttons[10]!.value;
    curStates.RThumb = gamepad.value.buttons[11]!.value;
    curStates.Menu = gamepad.value.buttons[16]!.value;
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
  connectEvent(enabled.value);
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
        <label class="subtitle mr-2">Connection</label>
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
          v-for="(value, name) of curStates"
          :key="name"
          class="ml-0 flex w-[8ex] flex-col text-center"
        >
          <p class="subtitle m-0">
            {{ name }}
          </p>
          {{ /X|Y|Z$/.test(name) ? value.toFixed(4) : value.toFixed(0) }}
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
