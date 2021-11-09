<template>
  <div class="component">
    <div class="card">


      <div class="row" style="margin-right: 0; align-items: center;">
        <div class="header">
          <h2>{{ deviceName }} WebGamepad</h2>
<!--           <span v-if="connected" class="pill green">Connected</span>
          <span v-else class="pill">Disconnected</span> -->
        </div>

        <div class="row" style="justify-content: flex-end; flex-grow: 1; margin-right: 0">
          <div class="column">
            <label class="subtitle">Gamepad Connection</label>
            <RadioButtons
              :options="['Disable', 'Enable']"
              defaultOption="Disable"
              v-on:selectOption="self.enabled = $event === 'Enable'"
            />
          </div>
        </div>
      </div>


      <div class="row" v-if="connected">
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
export default class WebGamepad extends Vue {
  @Prop() controllerName!: string;
  @Prop() controllerStatus!: number;

  gamepad = navigator.getGamepads()[0];
  gamepadName = "Waiting for gamepad...";
  gamepadConnected = false;
  enabledBool = false;
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

  tick(instance: WebGamepad): void {
    if (!instance.enabled) {
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


  // get controls(): string[][] {
  //   const controlOrder = ["AbsoluteX", "AbsoluteY", "AbsoluteRX", "AbsoluteRY", "AbsoluteZ", "AbsoluteRZ", "AbsoluteHat0X", "AbsoluteHat0Y", "ButtonSouth", "ButtonEast", "ButtonWest", "ButtonNorth", "ButtonLT", "ButtonRT", "ButtonLThumb", "ButtonRThumb", "ButtonSelect", "ButtonStart", "ButtonMenu", "ButtonEStop"];
  //   var controls = [];
  //   for (const ctrl of controlOrder) {
  //     var value = this.getValue(ctrl);
  //     if (value != "") {
  //       controls.push([ctrl.replace("Absolute", "").replace("Button", ""), value]);
  //     }
  //   }
  //   return controls;
  // }


  set enabled (opt: boolean) {
    console.log("SMURF10: " + opt);
    var prev = this.enabledBool;
    this.enabledBool = opt;
    if (opt && !prev) {
      //this.sendConnectionStatus(true);
      this.tick(this);
    }else if (!opt && prev){
      //this.sendConnectionStatus(false);
    }
  }

  get enabled (): boolean {
    return this.enabledBool;
  }

  get connected (): boolean {
    return true;
    // if (this.enabled === true && this.gamepad != null) {
    //   console.log("SMURF1");
    //   return this.gamepad.connected;
    // }
    // console.log("SMURF2");
    // return false;
  }

  get deviceName (): string {
    return this.gamepadName;
  }

  // Mappings
  getAxis(instance: WebGamepad, axis: number): number {
    if (instance.gamepad) {
      return instance.gamepad.axes[axis]
    }
    return NaN
  }

  getBtn(instance: WebGamepad, btn: number): number {
    if (instance.gamepad) {
      return instance.gamepad.buttons[btn].value
    }
    return NaN
  }

  // Axes
  get X (): number {
    return this.getAxis(this, 0);
  }

  get Y (): number {
    return this.getAxis(this, 1); 
  }

  get RX (): number {
    return this.getAxis(this, 2); 
  }

  get RY (): number {
    return this.getAxis(this, 3); 
  }

  get Z (): number {
    return this.getBtn(this, 6); 
  }

  get RZ (): number {
    return this.getBtn(this, 7); 
  }

  get HatX (): number {
    var ret = 0
    this.getBtn(this, 14) === 1 ? ret = -1 : ret;
    this.getBtn(this, 15) === 1 ? ret = 1 : ret;
    return ret
  }

  get HatY (): number {
    var ret = 0
    this.getBtn(this, 12) === 1 ? ret = -1 : ret;
    this.getBtn(this, 13) === 1 ? ret = 1 : ret;
    return ret
  }

  // Buttons
  get South (): number {
    return this.getBtn(this, 0);
  }

  get East (): number {
    return this.getBtn(this, 1);
  }

  get West (): number {
    return this.getBtn(this, 2);
  }

  get North (): number {
    return this.getBtn(this, 3);
  }

  get LT (): number {
    return this.getBtn(this, 4);
  }

  get RT (): number {
    return this.getBtn(this, 5);
  }

  get Select (): number {
    return this.getBtn(this, 8);
  }

  get Start (): number {
    return this.getBtn(this, 9);
  }

  get LThumb (): number {
    return this.getBtn(this, 10);
  }

  get RThumb (): number {
    return this.getBtn(this, 11);
  }

  get Menu (): number {
    return this.getBtn(this, 16);
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
