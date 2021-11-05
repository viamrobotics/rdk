<template>
  <div class="component">
    <div class="card">


      <div class="row" style="margin-right: 0; align-items: center;">
        <div class="header">
          <h2>{{ self.deviceName }}</h2>
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
    //this.tick()
  }


  // sendEvent(): void{
  //   newEvent = new InputControllerEvent();
  //   req = new InputControllerInjectEventRequest();
  //   req.setController(this.controller);
  //   req.setEvent(newEvent);
  // }

  tick(instance: Gamepad): void {
    if (!instance.useWeb) {
      console.log("SMURFX");
      return;
    }
    var found = false;
    const pads = navigator.getGamepads();
    for (const g of pads) {
      if (g != null) {
        found = true;
        instance.gamepad = g;
        instance.gamepadName = "Waiting for gamepad...";
        instance.gamepadConnected = true;
        if (instance.gamepad.mapping === "standard"){
          instance.gamepadName = instance.gamepad.id.replace(/ \(STANDARD .*\)/i, "");
        }else{
          instance.gamepadName = instance.gamepad.id
        }
        break;
      }
    }
    if (!found) {
      instance.gamepad = null;
      instance.gamepadConnected = false;      
    }
    window.requestAnimationFrame(() => instance.tick(instance));
  }



  set useWeb (opt: boolean) {
    var prev = this.useWebBool;
    this.useWebBool = opt;
    if (opt && !prev) {
      //this.sendConnectionStatus(true);
      window.requestAnimationFrame(() => this.tick(this));
    }else if (!opt && prev){
      //this.sendConnectionStatus(false);
    }
  }

  get useWeb (): boolean {
    return this.useWebBool;
  }

  get connected (): boolean {
    if (this.useWebBool === true) {
      return this.gamepad ? this.gamepad.connected : false;
    }
    console.log(this.controllerStatus);
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
  getAxis(instance: Gamepad, axis: number): number {
    if (instance.gamepad) {
      return instance.gamepad.axes[axis]
    }
    return NaN
  }

  getBtn(instance: Gamepad, btn: number): number {
    if (instance.gamepad) {
      return instance.gamepad.buttons[btn].value
    }
    return NaN
  }

  getRemoteValue(instance: Gamepad, ctrl: string): number {
    for (const stat of instance.controllerStatus.eventsList) {
      if (stat.control === ctrl) {
        return stat.value;
      }
    }
    return NaN;
  }

  // Axes
  get X (): number {
    return this.useWeb ? this.getAxis(this, 0) : this.getRemoteValue(this, "AbsoluteX");
  }

  get Y (): number {
    return this.useWeb ? this.getAxis(this, 1): this.getRemoteValue(this, "AbsoluteY"); 
  }

  get RX (): number {
    return this.useWeb ? this.getAxis(this, 2): this.getRemoteValue(this, "AbsoluteRX"); 
  }

  get RY (): number {
    return this.useWeb ? this.getAxis(this, 3): this.getRemoteValue(this, "AbsoluteRY"); 
  }

  get Z (): number {
    return this.useWeb ? this.getBtn(this, 6): this.getRemoteValue(this, "AbsoluteZ"); 
  }

  get RZ (): number {
    return this.useWeb ? this.getBtn(this, 7): this.getRemoteValue(this, "AbsoluteRZ"); 
  }

  get HatX (): number {
    if (this.useWeb === false) {
      return this.getRemoteValue(this, "AbsoluteHat0X");
    }
    var ret = 0
    this.getBtn(this, 14) === 1 ? ret = -1 : ret;
    this.getBtn(this, 15) === 1 ? ret = 1 : ret;
    return ret
  }

  get HatY (): number {
    if (this.useWeb === false) {
      return this.getRemoteValue(this, "AbsoluteHat0Y");
    }
    var ret = 0
    this.getBtn(this, 12) === 1 ? ret = -1 : ret;
    this.getBtn(this, 13) === 1 ? ret = 1 : ret;
    return ret
  }
  

  // Buttons
  get South (): number {
    return this.useWeb ? this.getBtn(this, 0): this.getRemoteValue(this, "ButtonSouth");
  }

  get East (): number {
    return this.useWeb ? this.getBtn(this, 1): this.getRemoteValue(this, "ButtonEast");
  }

  get West (): number {
    return this.useWeb ? this.getBtn(this, 2): this.getRemoteValue(this, "ButtonWest");
  }

  get North (): number {
    return this.useWeb ? this.getBtn(this, 3): this.getRemoteValue(this, "ButtonNorth");
  }

  get LT (): number {
    return this.useWeb ? this.getBtn(this, 4): this.getRemoteValue(this, "ButtonLT");
  }

  get RT (): number {
    return this.useWeb ? this.getBtn(this, 5): this.getRemoteValue(this, "ButtonRT");
  }

  get Select (): number {
    return this.useWeb ? this.getBtn(this, 8): this.getRemoteValue(this, "ButtonSelect");
  }

  get Start (): number {
    return this.useWeb ? this.getBtn(this, 9): this.getRemoteValue(this, "ButtonStart");
  }

  get LThumb (): number {
    return this.useWeb ? this.getBtn(this, 10): this.getRemoteValue(this, "ButtonLThumb");
  }

  get RThumb (): number {
    return this.useWeb ? this.getBtn(this, 11): this.getRemoteValue(this, "ButtonRThumb");
  }

  get Menu (): number {
    return this.useWeb ? this.getBtn(this, 16): this.getRemoteValue(this, "ButtonMenu");
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
