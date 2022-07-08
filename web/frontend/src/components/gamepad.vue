<script setup lang="ts">

import { ref, watch, onMounted } from 'vue';
import { Timestamp } from 'google-protobuf/google/protobuf/timestamp_pb';
import InputController from '../gen/proto/api/component/inputcontroller/v1/input_controller_pb.esm';

interface Emits {
  (event: 'execute', req: unknown): void
}

const emit = defineEmits<Emits>()


const gamepad = ref(navigator.getGamepads()[0]);
const gamepadName = ref('Waiting for gamepad...');
const gamepadConnected = ref(false);
const gamepadConnectedPrev = ref(false);
const enabled = ref(false);

watch(() => enabled, () => {
  connectEvent(enabled.value);
});

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

let prevStates: Record<string, number> = {}

onMounted(() => {
  prevStates = { ...prevStates, ...curStates };
  tick();
});

const processEvents = () => {
  if (gamepadConnected.value === false) {
    for (const ctrl in curStates) {
      curStates[ctrl] = Number.NaN;
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

  for (const ctrl in curStates) {
    if (
      curStates[ctrl] === prevStates[ctrl] ||
      (Number.isNaN(curStates[ctrl]) &&
        Number.isNaN(prevStates[ctrl]))
    ) {
      continue;
    }
    const newEvent = new InputController.Event();
    newEvent.setTime(Timestamp.fromDate(new Date()));
    if (/X|Y|Z$/.test(ctrl)) {
      newEvent.setControl(`Absolute${ctrl}`);
      newEvent.setEvent('PositionChangeAbs');
    } else {
      newEvent.setControl(`Button${ctrl}`);
      newEvent.setEvent(
        curStates[ctrl] ? 'ButtonPress' : 'ButtonRelease'
      );
    }

    if (Number.isNaN(curStates[ctrl])) {
      newEvent.setEvent('Disconnect');
      newEvent.setValue(0);
    } else {
      newEvent.setValue(curStates[ctrl]);
    }
    sendEvent(newEvent);
  }
};

const connectEvent = (con: boolean) => {
  if (
    (con === true && gamepadConnected.value === false) ||
    (con === false && gamepadConnectedPrev.value === false)
  ) {
    return;
  }

  for (const ctrl in curStates) {
    const newEvent = new InputController.Event();
    newEvent.setTime(Timestamp.fromDate(new Date()));
    newEvent.setEvent(con ? 'Connect' : 'Disconnect');
    newEvent.setValue(0);
    if (/X|Y|Z$/.test(ctrl)) {
      newEvent.setControl(`Absolute${ ctrl}`);
    } else {
      newEvent.setControl(`Button${ ctrl}`);
    }
    sendEvent(newEvent);
  }
};

const sendEvent = (newEvent: InputController.Event) => {
  if (enabled.value) {
    const req = new InputController.TriggerEventRequest();
    req.setController('WebGamepad');
    req.setEvent(newEvent);
    emit('execute', req);
  }
};

const tick = () => {
  let gamepadFound = false;
  const pads = navigator.getGamepads();
  for (const g of pads) {
    if (g) {
      gamepad.value = g;
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
      ? gamepad.value.id.replace(/ \(standard .*\)/i, '')
      : gamepad.value.id;

    prevStates = { ...prevStates, ...curStates };
    gamepadConnected.value = gamepad.value.connected;

    curStates['X'] = gamepad.value.axes[0];
    curStates['Y'] = gamepad.value.axes[1];
    curStates['RX'] = gamepad.value.axes[2];
    curStates['RY'] = gamepad.value.axes[3];
    curStates['Z'] = gamepad.value.buttons[6].value;
    curStates['RZ'] = gamepad.value.buttons[7].value;
    curStates['Hat0X'] = gamepad.value.buttons[14].value * -1 + gamepad.value.buttons[15].value;
    curStates['Hat0Y'] = gamepad.value.buttons[12].value * -1 + gamepad.value.buttons[13].value;
    curStates['South'] = gamepad.value.buttons[0].value;
    curStates['East'] = gamepad.value.buttons[1].value;
    curStates['West'] = gamepad.value.buttons[2].value;
    curStates['North'] = gamepad.value.buttons[3].value;
    curStates['LT'] = gamepad.value.buttons[4].value;
    curStates['RT'] = gamepad.value.buttons[5].value;
    curStates['Select'] = gamepad.value.buttons[8].value;
    curStates['Start'] = gamepad.value.buttons[9].value;
    curStates['LThumb'] = gamepad.value.buttons[10].value;
    curStates['RThumb'] = gamepad.value.buttons[11].value;
    curStates['Menu'] = gamepad.value.buttons[16].value;
  }

  processEvents();
  window.requestAnimationFrame(tick);
};

</script>

<template>
  <div class="component">
    <div class="card">
      <div
        class="row"
        style="margin-right: 0; align-items: center"
      >
        <div class="header">
          <h2>WebGamepad</h2>
          <span
            v-if="gamepadConnected && enabled"
            class="pill green"
          >Connected</span>
          <span
            v-else
            class="pill"
          >Disconnected</span>
        </div>

        <div
          class="row"
          style="justify-content: flex-end; flex-grow: 1; margin-right: 0"
        >
          <div class="column">
            <label class="subtitle">Connection</label>
            <v-switch
              :value="enabled ? 'on' : 'off'"
              @input="enabled = !enabled"
            />
          </div>
        </div>
      </div>

      <div
        v-if="gamepadConnected"
        class="row"
      >
        <div
          v-for="(value, name) of curStates"
          :key="name"
          class="column control"
        >
          <p class="subtitle">
            {{ name }}
          </p>
          {{ /X|Y|Z$/.test(name) ? value.toFixed(4) : value.toFixed(0) }}
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
p,
h2,
h3 {
  margin: 0;
}

.header {
  display: flex;
  flex-direction: row;
  align-items: center;
  align-content: center;
  gap: 8px;
}

.row {
  display: flex;
  flex-direction: row;
  margin-right: 12px;
  gap: 8px;
  margin-bottom: 12px;
}

.subtitle {
  color: var(--black-70);
}

.column {
  display: flex;
  flex-direction: column;
  margin-left: 0px;
}

.control {
  width: 8ex;
}

.margin-bottom {
  margin-bottom: 32px;
}
</style>
