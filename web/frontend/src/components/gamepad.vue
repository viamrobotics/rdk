<!-- eslint-disable id-length -->
<script setup lang="ts">

import { grpc } from '@improbable-eng/grpc-web';
import { onMounted, onUnmounted, watch } from 'vue';
import { Timestamp } from 'google-protobuf/google/protobuf/timestamp_pb';
import InputController from '../gen/component/inputcontroller/v1/input_controller_pb.esm';
import type { ServiceError } from '../gen/proto/stream/v1/stream_pb_service.esm';
import { toast } from '../lib/toast';

let gamepadIdx = $ref<number | null>(null);
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

let lastError = Date.now();
const sendEvent = (newEvent: InputController.Event) => {
  if (!enabled) {
    return;
  }
  const req = new InputController.TriggerEventRequest();
  req.setController('WebGamepad');
  req.setEvent(newEvent);
  window.inputControllerService.triggerEvent(req, new grpc.Metadata(), (error: ServiceError | null) => {
    if (error) {
      const now = Date.now();
      if (now - lastError > 1000) {
        lastError = now;
        toast.error(error.message);
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

const currentGamepad = () => {
  return gamepadIdx === null ? null : navigator.getGamepads()[gamepadIdx];
};

const connectEvent = (con: boolean) => {
  const gamepad = currentGamepad();
  if (
    (con && (!gamepad || !gamepad.connected)) ||
    (!con && !gamepadConnectedPrev)
  ) {
    return;
  }

  const nowTS = nextTS();
  try {
    for (const ctrl of Object.keys(curStates)) {
      const newEvent = new InputController.Event();
      nowTS.setNanos(nowTS.getNanos() + 1);
      newEvent.setTime(nowTS);
      newEvent.setEvent(con ? 'Connect' : 'Disconnect');
      newEvent.setValue(0);

      if ((/X|Y|Z$/u).test(ctrl)) {
        newEvent.setControl(`Absolute${ctrl}`);
      } else {
        newEvent.setControl(`Button${ctrl}`);
      }

      sendEvent(newEvent);
    }
  } finally {
    lastTS = nowTS;
  }
};

const processEvents = (connected: boolean) => {
  if (!connected) {
    for (const key of Object.keys(curStates)) {
      curStates[key] = Number.NaN;
    }

    if (gamepadConnectedPrev) {
      connectEvent(false);
      gamepadConnectedPrev = false;
    }
    return;
  } else if (!gamepadConnectedPrev) {
    connectEvent(true);
    gamepadConnectedPrev = true;
  }

  const nowTS = nextTS();

  try {
    for (const [key, value] of Object.entries(curStates)) {
      if (value === prevStates[key] || (Number.isNaN(value) && Number.isNaN(prevStates[key]))) {
        continue;
      }
      const newEvent = new InputController.Event();
      nowTS.setNanos(nowTS.getNanos() + 1);
      newEvent.setTime(nowTS);
      if ((/X|Y|Z$/u).test(key)) {
        newEvent.setControl(`Absolute${key}`);
        newEvent.setEvent('PositionChangeAbs');
      } else {
        newEvent.setControl(`Button${key}`);
        newEvent.setEvent(value ? 'ButtonPress' : 'ButtonRelease');
      }

      if (Number.isNaN(value)) {
        newEvent.setEvent('Disconnect');
        newEvent.setValue(0);
      } else {
        newEvent.setValue(value);
      }
      sendEvent(newEvent);
    }
  } finally {
    lastTS = nowTS;
  }
};

const tick = () => {
  const gamepad = currentGamepad();
  if (!gamepad || !gamepad.connected) {
    if (enabled) {
      processEvents(false);
    }
    return;
  }

  prevStates = { ...prevStates, ...curStates };

  // eslint-disable-next-line unicorn/no-unsafe-regex
  const re = /^-?\d+(?:.\d{0,4})?/u;
  const trunc = (val: number): number => {
    if (Number.isNaN(val)) {
      return val;
    }
    const match = val.toString().match(re);
    if (match && match.length === 0) {
      return val;
    }
    return Number(match![0]!);
  };

  curStates.X = trunc(gamepad.axes[0]!);
  curStates.Y = trunc(gamepad.axes[1]!);
  curStates.RX = trunc(gamepad.axes[2]!);
  curStates.RY = trunc(gamepad.axes[3]!);
  curStates.Z = trunc(gamepad.buttons[6]!.value);
  curStates.RZ = trunc(gamepad.buttons[7]!.value);
  curStates.Hat0X = trunc((gamepad.buttons[14]!.value * -1) + gamepad.buttons[15]!.value);
  curStates.Hat0Y = trunc((gamepad.buttons[12]!.value * -1) + gamepad.buttons[13]!.value);
  curStates.South = trunc(gamepad.buttons[0]!.value);
  curStates.East = trunc(gamepad.buttons[1]!.value);
  curStates.West = trunc(gamepad.buttons[2]!.value);
  curStates.North = trunc(gamepad.buttons[3]!.value);
  curStates.LT = trunc(gamepad.buttons[4]!.value);
  curStates.RT = trunc(gamepad.buttons[5]!.value);
  curStates.Select = trunc(gamepad.buttons[8]!.value);
  curStates.Start = trunc(gamepad.buttons[9]!.value);
  curStates.LThumb = trunc(gamepad.buttons[10]!.value);
  curStates.RThumb = trunc(gamepad.buttons[11]!.value);
  curStates.Menu = trunc(gamepad.buttons[16]!.value);

  if (enabled) {
    processEvents(true);
  }

  handle = window.setTimeout(tick, 10);
};

onMounted(() => {
  window.addEventListener('gamepadconnected', (event) => {
    if (gamepadIdx) {
      return;
    }
    gamepadIdx = event.gamepad.index;
    tick();
  });
  window.addEventListener('gamepaddisconnected', (event) => {
    if (gamepadIdx === event.gamepad.index || !currentGamepad()?.connected) {
      gamepadIdx = null;
    }
  });
  // initial search
  const pads = navigator.getGamepads();
  for (const pad of pads) {
    if (pad) {
      gamepadIdx = pad.index;
      break;
    }
  }

  if (!gamepadIdx) {
    return;
  }
  prevStates = { ...prevStates, ...curStates };
  tick();
});

onUnmounted(() => {
  clearTimeout(handle);
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
    <span
      v-if="currentGamepad()?.connected"
      slot="title"
    > ({{ currentGamepad()?.id }})</span>
    <div slot="header">
      <span
        v-if="currentGamepad()?.connected && enabled"
        class="rounded-full bg-green-500 px-3 py-0.5 text-xs text-white"
      >Enabled</span>
      <span
        v-else
        class="rounded-full bg-gray-200 px-3 py-0.5 text-xs text-gray-800"
      >Disabled</span>
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
        v-if="currentGamepad()?.connected"
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
