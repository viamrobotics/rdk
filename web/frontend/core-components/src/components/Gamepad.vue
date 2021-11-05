<template>
  <div class="component">
    <div class="card">


      <div class="row" style="margin-right: 0; align-items: center;">
        <div class="header">
          <h2>{{ deviceName }}</h2>
          <span v-if="self.connected" class="pill green">Connected</span>
          <span v-else class="pill">Disconnected</span>
        </div>

        <div class="row" style="justify-content: flex-end; flex-grow: 1; margin-right: 0">
          <div class="column">
            <label class="subtitle">Gamepad Connection</label>
            <RadioButtons
              :options="['Direct', 'Web']"
              defaultOption="Direct"
              v-on:selectOption="self.useWeb = $event === 'Web'"
            />
          </div>
        </div>
      </div>


      <div class="row" style="justify-content: space-between">
        <div class="row" v-if="self.connected">
          <div v-for="axis in axes" :key="axis" class="column axis">
            <p class="subtitle">{{ axis }}</p>
            {{ self[axis].toFixed(4) }}
          </div>
          <div v-for="button in buttons" :key="button" class="column button">
            <p class="subtitle">{{ button }}</p>
            {{ self[button].toFixed(0) }}
          </div>
        </div>
      </div>


    </div>
  </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";

import {
  InputControllerInjectEventRequest,
  InputControllerEvent,
  InputControllerStatus,
} from "proto/robot_pb";

import RadioButtons from "./RadioButtons.vue";

@Component({
  components: {
    RadioButtons,
  },
})
export default class Gamepad extends Vue {
  @Prop() controllerName!: string;
  @Prop() controllerStatus!: InputControllerStatus.AsObject;

  gamepad = navigator.getGamepads()[0];
  gamepadName = "Waiting for gamepad...";
  gamepadConnected = false;
  useWebBool = true;
  self = this;

  axes = ["X", "Y", "RX", "RY", "Z", "RZ", "HatX", "HatY"];
  buttons = ["South", "East", "West", "North", "LT", "RT", "LThumb", "RThumb", "Select", "Start", "Menu"];

  mounted(): void {
    this.tick()
  }


  // sendEvent(): void{
  //   newEvent = new InputControllerEvent();
  //   req = new InputControllerInjectEventRequest();
  //   req.setController(this.controller);
  //   req.setEvent(newEvent);
  // }

  tick(): void {
    if (!this.useWeb) {
      console.log("SMURFX");
      return;
    }
    console.log("SMURF2" + this.gamepad);
    if (this.gamepad && this.gamepad.connected) {
      this.gamepadConnected = true;
    } else {
      this.gamepad = null;
      this.gamepadConnected = false;
      const pads = navigator.getGamepads();
      for (const g of pads) {
        if (g != null) {
          this.gamepad = g;
          this.gamepadName = "Waiting for gamepad...";
          this.gamepadConnected = true;
          if (this.gamepad.mapping === "standard"){
            this.gamepadName = this.gamepad.id.replace(/ \(STANDARD .*\)/i, "");
          }else{
            this.gamepadName = this.gamepad.id
          }
          break;
        }
      }
    }
    window.requestAnimationFrame(() => this.tick());
  }



  set useWeb (opt: boolean) {
    var prev = this.useWebBool;
    this.useWebBool = opt;
    if (opt && !prev) {
      //this.sendConnectionStatus(true);
      this.tick();
    }else if (!opt && prev){
      //this.sendConnectionStatus(false);
    }
  }

  get useWeb (): boolean {
    return this.useWebBool;
  }

  get connected (): boolean {
    if (this.useWebBool === true) {
      return this.gamepadConnected;
    }
    if (this.controllerStatus.eventsList[0] && this.controllerStatus.eventsList[0].event != "Disconnect") {
      return true
    }else{
      return false
    }
  }

  get deviceName (): string {
    return this.useWeb ? this.gamepadName : this.controllerName;
  }

  // Mappings
  private getAxis(axis: number): number {
    if (this.gamepad) {
      return this.gamepad.axes[axis]
    }
    return NaN
  }

  private getBtn(btn: number): number {
    if (this.gamepad) {
      return this.gamepad.buttons[btn].value
    }
    return NaN
  }

  private getRemoteValue(ctrl: string): number {
    for (const stat of this.controllerStatus.eventsList) {
      if (stat.control === ctrl) {
        return stat.value;
      }
    }
    return NaN;
  }

  // Axes
  get X (): number {
    return this.useWeb ? this.getAxis(0) : this.getRemoteValue("AbsoluteX");
  }

  get Y (): number {
    return this.useWeb ? this.getAxis(1): this.getRemoteValue("AbsoluteY"); 
  }

  get RX (): number {
    return this.useWeb ? this.getAxis(2): this.getRemoteValue("AbsoluteRX"); 
  }

  get RY (): number {
    return this.useWeb ? this.getAxis(3): this.getRemoteValue("AbsoluteRY"); 
  }

  get Z (): number {
    return this.useWeb ? this.getBtn(6): this.getRemoteValue("AbsoluteZ"); 
  }

  get RZ (): number {
    return this.useWeb ? this.getBtn(7): this.getRemoteValue("AbsoluteRZ"); 
  }

  get HatX (): number {
    if (this.useWeb === false) {
      return this.getRemoteValue("AbsoluteHat0X");
    }
    var ret = 0
    this.getBtn(14) === 1 ? ret = -1 : ret;
    this.getBtn(15) === 1 ? ret = 1 : ret;
    return ret
  }

  get HatY (): number {
    if (this.useWeb === false) {
      return this.getRemoteValue("AbsoluteHat0Y");
    }
    var ret = 0
    this.getBtn(12) === 1 ? ret = -1 : ret;
    this.getBtn(13) === 1 ? ret = 1 : ret;
    return ret
  }
  

  // Buttons
  get South (): number {
    return this.useWeb ? this.getBtn(0): this.getRemoteValue("ButtonSouth");
  }

  get East (): number {
    return this.useWeb ? this.getBtn(1): this.getRemoteValue("ButtonEast");
  }

  get West (): number {
    return this.useWeb ? this.getBtn(2): this.getRemoteValue("ButtonWest");
  }

  get North (): number {
    return this.useWeb ? this.getBtn(3): this.getRemoteValue("ButtonNorth");
  }

  get LT (): number {
    return this.useWeb ? this.getBtn(4): this.getRemoteValue("ButtonLT");
  }

  get RT (): number {
    return this.useWeb ? this.getBtn(5): this.getRemoteValue("ButtonRT");
  }

  get Select (): number {
    return this.useWeb ? this.getBtn(8): this.getRemoteValue("ButtonSelect");
  }

  get Start (): number {
    return this.useWeb ? this.getBtn(9): this.getRemoteValue("ButtonStart");
  }

  get LThumb (): number {
    return this.useWeb ? this.getBtn(10): this.getRemoteValue("ButtonLThumb");
  }

  get RThumb (): number {
    return this.useWeb ? this.getBtn(11): this.getRemoteValue("ButtonRThumb");
  }

  get Menu (): number {
    return this.useWeb ? this.getBtn(16): this.getRemoteValue("ButtonMenu");
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

.axis {
  width: 7ex;
}

.margin-bottom {
  margin-bottom: 32px;
}
</style>
