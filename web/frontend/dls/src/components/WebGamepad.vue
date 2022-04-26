<template>
  <div class="component">
    <div class="card">
      <div class="row" style="margin-right: 0; align-items: center">
        <div class="header">
          <h2>{{ ControllerName }} WebGamepad</h2>
          <span v-if="gamepadConnected && enabled" class="pill green"
            >Connected</span
          >
          <span v-else class="pill">Disconnected</span>
        </div>

        <div
          class="row"
          style="justify-content: flex-end; flex-grow: 1; margin-right: 0"
        >
          <div class="column">
            <label class="subtitle">Connection</label>
            <RadioButtons
              :options="['Disable', 'Enable']"
              defaultOption="Disable"
              v-on:selectOption="enabled = $event === 'Enable'"
            />
          </div>
        </div>
      </div>

      <div class="row" v-if="gamepadConnected">
        <div
          v-for="(value, name) of curStates"
          :key="name"
          class="column control"
        >
          <p class="subtitle">{{ name }}</p>
          {{ /X|Y|Z$/.test(name) ? value.toFixed(4) : value.toFixed(0) }}
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { Component, Vue } from "vue-property-decorator";
import { Timestamp } from "google-protobuf/google/protobuf/timestamp_pb";

import {
  TriggerEventRequest,
  Event,
} from "proto/api/component/inputcontroller/v1/input_controller_pb";

import RadioButtons from "./RadioButtons.vue";

@Component({
  components: {
    RadioButtons,
  },
})
export default class WebGamepad extends Vue {
  gamepad = navigator.getGamepads()[0];
  gamepadState = null;
  gamepadName = "Waiting for gamepad...";
  gamepadConnected = false;
  gamepadConnectedPrev = false;
  enabledBool = false;

  get enabled(): boolean {
    return this.enabledBool;
  }

  set enabled(enable: boolean) {
    if (enable === true) {
      this.enabledBool = true;
      this.connectEvent(true);
    } else {
      this.connectEvent(false);
      this.enabledBool = enable;
    }
  }

  curStates = {
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
  } as { [key: string]: number };

  prevStates = {} as { [key: string]: number };

  mounted(): void {
    this.prevStates = Object.assign(this.prevStates, this.curStates);
    this.tick();
  }

  processEvents(): void {
    if (this.gamepadConnected === false) {
      for (const ctrl in this.curStates) {
        this.curStates[ctrl] = Number.NaN;
      }
      if (this.gamepadConnectedPrev === true) {
        this.connectEvent(false);
        this.gamepadConnectedPrev = false;
      }
      return;
    } else if (this.gamepadConnectedPrev === false) {
      this.connectEvent(true);
      this.gamepadConnectedPrev = true;
    }

    for (const ctrl in this.curStates) {
      if (
        this.curStates[ctrl] === this.prevStates[ctrl] ||
        (Number.isNaN(this.curStates[ctrl]) &&
          Number.isNaN(this.prevStates[ctrl]))
      ) {
        continue;
      }
      const newEvent = new Event();
      newEvent.setTime(Timestamp.fromDate(new Date()));
      if (/X|Y|Z$/.test(ctrl)) {
        newEvent.setControl(`Absolute${ctrl}`);
        newEvent.setEvent("PositionChangeAbs");
      } else {
        newEvent.setControl(`Button${ctrl}`);
        newEvent.setEvent(
          this.curStates[ctrl] ? "ButtonPress" : "ButtonRelease"
        );
      }

      if (Number.isNaN(this.curStates[ctrl])) {
        newEvent.setEvent("Disconnect");
        newEvent.setValue(0);
      } else {
        newEvent.setValue(this.curStates[ctrl]);
      }
      this.sendEvent(newEvent);
    }
  }

  connectEvent(con: boolean): void {
    if (
      (con === true && this.gamepadConnected === false) ||
      (con === false && this.gamepadConnectedPrev === false)
    ) {
      return;
    }

    for (const ctrl in this.curStates) {
      const newEvent = new Event();
      newEvent.setTime(Timestamp.fromDate(new Date()));
      newEvent.setEvent(con ? "Connect" : "Disconnect");
      newEvent.setValue(0);
      if (/X|Y|Z$/.test(ctrl)) {
        newEvent.setControl(`Absolute${ctrl}`);
      } else {
        newEvent.setControl(`Button${ctrl}`);
      }
      this.sendEvent(newEvent);
    }
  }

  sendEvent(newEvent: Event): void {
    if (this.enabled) {
      const request = new TriggerEventRequest();
      request.setController("WebGamepad");
      request.setEvent(newEvent);
      this.$emit("execute", request);
    }
  }

  grpcCallback(error: Error): void {
    if (error) {
      console.log(error);
    }
  }

  tick(): void {
    let gamepadFound = false;
    const pads = navigator.getGamepads();
    for (const g of pads) {
      if (g) {
        this.gamepad = g;
        gamepadFound = true;
        break;
      }
    }
    if (gamepadFound === false) {
      this.gamepadName = "Waiting for gamepad...";
      this.gamepadConnected = false;
      this.gamepad = null;
    }

    if (this.gamepad) {
      this.gamepadName = this.gamepad.mapping === "standard" ? this.gamepad.id.replace(/ \(standard .*\)/i, "") : this.gamepad.id;

      this.prevStates = Object.assign(this.prevStates, this.curStates);
      this.gamepadConnected = this.gamepad.connected;

      this.curStates["X"] = this.gamepad.axes[0];
      this.curStates["Y"] = this.gamepad.axes[1];
      this.curStates["RX"] = this.gamepad.axes[2];
      this.curStates["RY"] = this.gamepad.axes[3];
      this.curStates["Z"] = this.gamepad.buttons[6].value;
      this.curStates["RZ"] = this.gamepad.buttons[7].value;
      this.curStates["Hat0X"] =
        this.gamepad.buttons[14].value * -1 + this.gamepad.buttons[15].value;
      this.curStates["Hat0Y"] =
        this.gamepad.buttons[12].value * -1 + this.gamepad.buttons[13].value;
      this.curStates["South"] = this.gamepad.buttons[0].value;
      this.curStates["East"] = this.gamepad.buttons[1].value;
      this.curStates["West"] = this.gamepad.buttons[2].value;
      this.curStates["North"] = this.gamepad.buttons[3].value;
      this.curStates["LT"] = this.gamepad.buttons[4].value;
      this.curStates["RT"] = this.gamepad.buttons[5].value;
      this.curStates["Select"] = this.gamepad.buttons[8].value;
      this.curStates["Start"] = this.gamepad.buttons[9].value;
      this.curStates["LThumb"] = this.gamepad.buttons[10].value;
      this.curStates["RThumb"] = this.gamepad.buttons[11].value;
      this.curStates["Menu"] = this.gamepad.buttons[16].value;
    }

    this.processEvents();
    window.requestAnimationFrame(() => this.tick());
  }
}
</script>

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
