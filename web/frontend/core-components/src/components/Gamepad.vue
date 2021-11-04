<template>
  <div class="component">
    <div class="card">


      <div class="row" style="margin-right: 0; align-items: center;">
        <div class="header">
          <h2>{{ gamepadName }}</h2>
          <span v-if="gamepad" class="pill green">Connected</span>
          <span v-else class="pill">Disconnected</span>
        </div>

        <div class="row" style="justify-content: flex-end; flex-grow: 1; margin-right: 0">
          <button class="red" style="align-self: flex-end">
            <i class="far fa-times-circle"></i>
            STOP
          </button>
          <button class="green" style="align-self: flex-end">
            <i class="fas fa-play"></i>
            RUN
          </button>
        </div>
      </div>


      <div class="row" style="justify-content: space-between">
        <div class="row" v-if="gamepad">
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
import RadioButtons from "./RadioButtons.vue";

@Component({
  components: {
    RadioButtons,
  },
})
export default class Gamepad extends Vue {
  gamepad = navigator.getGamepads()[0]
  gamepadName = "Waiting for gamepad"

  self = this

  axes = ["X", "Y", "RX", "RY", "Z", "RZ", "HatX", "HatY"]
  buttons = ["South", "East", "West", "North", "LT", "RT", "LThumb", "RThumb", "Select", "Start", "Menu"]

  mounted(): void {
    this.tick()
  }

  tick(): void{
    this.gamepad = null;
    const pads = navigator.getGamepads();
    for (const g of pads) {
      if (g != null) {
        this.gamepad = g;
        break;
      }
    }
    if (this.gamepad === null) {
      this.gamepadName = "Waiting for gamepad...";
      window.requestAnimationFrame(() => this.tick());
      return;
    }
    if (this.gamepad.mapping === "standard"){
      this.gamepadName = this.gamepad.id.replace(/ \(STANDARD .*\)/i, "");
    }else{
      this.gamepadName = this.gamepad.id
    }
    window.requestAnimationFrame(() => this.tick());
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

  // Axes
  get X (): number {
    return this.getAxis(0);
  }

  get Y (): number {
    return this.getAxis(1);
  }

  get RX (): number {
    return this.getAxis(2);
  }

  get RY (): number {
    return this.getAxis(3);
  }

  get Z (): number {
    return this.getBtn(6);
  }

  get RZ (): number {
    return this.getBtn(7);
  }

  get HatX (): number {
    var ret = 0
    this.getBtn(14) === 1 ? ret = -1 : ret;
    this.getBtn(15) === 1 ? ret = 1 : ret;
    return ret
  }

  get HatY (): number {
    var ret = 0
    this.getBtn(12) === 1 ? ret = -1 : ret;
    this.getBtn(13) === 1 ? ret = 1 : ret;
    return ret
  }
  

  // Buttons
  get South (): number {
    return this.getBtn(0);
  }

  get East (): number {
    return this.getBtn(1);
  }

  get West (): number {
    return this.getBtn(2);
  }

  get North (): number {
    return this.getBtn(3);
  }

  get LT (): number {
    return this.getBtn(4);
  }

  get RT (): number {
    return this.getBtn(5);
  }

  get Select (): number {
    return this.getBtn(8);
  }

  get Start (): number {
    return this.getBtn(9);
  }

  get LThumb (): number {
    return this.getBtn(10);
  }

  get RThumb (): number {
    return this.getBtn(11);
  }

  get Menu (): number {
    return this.getBtn(16);
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
