<template>
  <div>
    <Collapse>
      <div class="flex float-left">
        <h2 class="p-4 text-xl">{{ baseName }}</h2>
        <Breadcrumbs :crumbs="crumbs" disabled="true"></Breadcrumbs>
        <div class="p-4 flex items-center flex-wrap" v-if="motorStatus.positionReporting">
          <p class="flex items-center border border-black rounded-full px-2 leading-tight">Position {{ motorStatus.position }}</p>
        </div>
        <div class="p-4 flex items-center flex-wrap">
          <ViamBadge color="green" v-if="motorStatus.isOn">Running</ViamBadge>
          <ViamBadge color="gray" v-if="!motorStatus.isOn">Idle</ViamBadge>
        </div>
      </div>
      <div class="p-2 float-right">
        <ViamButton color="danger" group variant="primary" @click="motorStop">
          <template v-slot:icon>
            <ViamIcon color="white" :path="mdiCloseOctagonOutline">STOP</ViamIcon>
          </template>
          STOP
        </ViamButton>
      </div>
      <template v-slot:content>
        <div
          class=""
          :style="{ height: maxHeight + 'px' }"
        >
          <div
            class="border border-black p-4 grid grid-cols-1"
            :style="{ maxHeight: maxHeight + 'px' }"
          >
            <div class="grid">
              <div
                class="column"
              >
                <p class="text-xs pb-2">Set Power</p>
                <RadioButtons
                  :options="['Go', 'Go To', 'Go For']"
                  defaultOption="Go"
                  :disabledOptions="[]"
                  v-on:selectOption="setMovementType($event)"
                />
              </div>
              <div
                class="flex pt-4"
                v-if="movementType === 'Go To'"
              >
                <div class="place-self-end pr-2">
                <span class="text-2xl">{{ movementType }}</span>
                <viam-info-button
                    class="pb-2"
                    :iconPath="mdiInformation"
                    :infoRows="infoGoTo"
                >
                </viam-info-button>
                </div>
                <ViamInput
                  type="number"
                  color="primary"
                  group="False"
                  variant="primary"
                  class="pr-2 w-48"
                  inputId="distance"
                  v-model="position"
                >
                  <span class="text-xs">Position in Revolutions</span>
                </ViamInput>
                <div class="column pr-4">
                  <p class="text-xs mb-1">Direction of Rotation</p>
                  <RadioButtons
                    :options="['Forwards', 'Backwards']"
                    defaultOption="Forwards"
                    :disabledOptions="[]"
                    v-on:selectOption="setDirection($event)"
                  />
                </div>
                <ViamInput
                  type="number"
                  color="primary"
                  group="False"
                  variant="primary"
                  class="pr-2 w-32"
                  inputId="distance"
                  v-model="rpm"
                >
                  <span class="text-xs">RPM</span>
                </ViamInput>
            </div>
              <div
                class="flex pt-4"
                v-if="movementType === 'Go For'"
              >
                <div class="place-self-end pr-2">
                <span class="text-2xl">{{ movementType }}</span>
                <viam-info-button
                    class="pb-2"
                    :iconPath="mdiInformation"
                    :infoRows="infoGoFor"
                >
                </viam-info-button>
                </div>
                <ViamInput
                  type="number"
                  color="primary"
                  group="False"
                  variant="primary"
                  class="pr-2 w-32"
                  inputId="distance"
                  v-model="revolutions"
                >
                  <span class="text-xs"># in Revolutions</span>
                </ViamInput>
                <div class="column pr-4">
                  <p class="text-xs mb-1">Direction of Rotation</p>
                  <RadioButtons
                    :options="['Forwards', 'Backwards']"
                    defaultOption="Forwards"
                    :disabledOptions="[]"
                    v-on:selectOption="setDirection($event)"
                  />
                </div>
                <ViamInput
                  type="number"
                  color="primary"
                  group="False"
                  variant="primary"
                  class="pr-2 w-32"
                  inputId="distance"
                  v-model="rpm"
                >
                  <span class="text-xs">RPM</span>
                </ViamInput>
            </div>
            <div
                class="flex items-start pt-4"
                v-if="movementType === 'Go'"
              >
                <div class="place-self-end pr-2">
                <span class="text-2xl">{{ movementType }}</span>
                <viam-info-button
                    class="pb-2"
                    :iconPath="mdiInformation"
                    :infoRows="infoGo"
                >
                </viam-info-button>
                </div>
                <div class="column pr-4">
                  <p class="text-xs pb-2 pt-1">Direction of Rotation</p>
                  <RadioButtons
                    :options="['Forwards', 'Backwards']"
                    defaultOption="Forwards"
                    :disabledOptions="[]"
                    v-on:selectOption="setDirection($event)"
                  />
                </div>
                <Range
                  class="pt-2"
                  id="power"
                  :min="0"
                  :max="100"
                  :step="25"
                  v-model="power"
                  unit="%"
                  name="Power %"
                ></Range>
              </div>
            </div>
            <div class="flex flex-row-reverse">
              <div>
                <ViamButton
                  color="success"
                  group
                  variant="primary"
                  @click="motorRun()"
                >
                  <template v-slot:icon>
                    <ViamIcon color="white" :path="mdiPlayCircleOutline">RUN</ViamIcon>
                  </template>
                  RUN
                </ViamButton>
              </div>
            </div>
          </div>
        </div>
      </template>
    </Collapse>
  </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";
import "vue-class-component/hooks";
import Collapse from "./Collapse.vue";
import Breadcrumbs from "./Breadcrumbs.vue";
import ViamIcon from "./ViamIcon.vue";
import {
  mdiRestore,
  mdiPlayCircleOutline,
  mdiCloseOctagonOutline,
  mdiAlertOctagonOutline
} from "@mdi/js";
import Tabs from "./Tabs.vue";
import Tab from "./Tab.vue";
import RadioButtons from "./RadioButtons.vue";
import ViamBadge from "./Badge.vue";
import Popper from "vue-popperjs";
import "vue-popperjs/dist/vue-popper.css";
import ViamButton from "./Button.vue";
import {
  SetPowerRequest,
  GoForRequest,
  GoToRequest,
  Status,
} from "proto/api/component/motor/v1/motor_pb";

@Component({
  components: {
    Collapse,
    Breadcrumbs,
    ViamIcon,
    RadioButtons,
    Tabs,
    Tab,
    ViamButton,
    Popper,
    ViamBadge,
  },
})
export default class MotorDetailNew extends Vue {
  @Prop({ default: null }) streamName!: string;
  @Prop({ default: null }) baseName!: string;
  @Prop({ default: null }) crumbs!: [string];
  @Prop() motorStatus!: Status.AsObject;

  mdiRestore = mdiRestore;
  mdiPlayCircleOutline = mdiPlayCircleOutline;
  mdiCloseOctagonOutline = mdiCloseOctagonOutline;
  mdiInformation = mdiAlertOctagonOutline;
  maxHeight = 500;
  selectedValue = "NoCamera";
  isContinuous = true;
  streamId = "stream-preview-" + this.streamName;
  selectedItem = "keyboard";
  pressedKey = 0;
  movementMode = "";
  movementType = "Go";
  direction: -1 | 1 = 1;
  spinType = "";
  position = 0;
  rpm = 0;
  power = 0;
  type = "go";
  speed = 0;
  revolutions = 0;
  infoGo = ["Continously moves"];
  infoGoTo = ["Relative to Home"];
  infoGoFor = ["Relative to where robot is currently is"];

  beforeMount(): void {
    window.addEventListener("resize", this.resizeContent);
  }

  beforeDestroy(): void {
    window.removeEventListener("resize", this.resizeContent);
  }

  mounted(): void {
    this.resizeContent();
  }

  setMovementMode(e: string): void {
    console.log(e);
    this.movementMode = e;
  }
  setMovementType(e: string): void {
    console.log(e);
    this.movementType = e;
    switch (this.movementType) {
      case "Go":
        this.type = "go";
        break;
      case "Go For":
        this.type = "goFor";
        break;
      case "Go To":
        this.type = "goTo";
        break;
    }
  }
  setSpinType(e: string): void {
    console.log(e);
    this.spinType = e;
  }
  setDirection(e: string): void {
    console.log(e);
    switch (e) {
      case "Forwards":
          this.direction = 1;
          break;
      case "Backwars":
          this.direction = -1;
          break;
      default:
          this.direction = 1;
    }

  }
  motorRun(): void {
    const command = this.asObject();
    console.log(command);
    this.$emit("motor-run", command);
  }
  motorStop(e: Event): void {
    e.preventDefault();
    e.stopPropagation();
    this.type = "go";
    this.position = 0;
    this.speed = 0;
    this.direction = 1;
    this.revolutions = 0;
    this.power = 0;
    const command = this.asObject();
    console.log(command);
    this.$emit("motor-stop", command);
  }
  resizeContent(): void {
      this.maxHeight = 250;
  }

  private validateRevolutions(revolutions: number): string {
    revolutions = Number.parseFloat(revolutions.toString());
    if (Number.isNaN(revolutions)) {
      return "Input is not a number";
    }
    return "";
  }

  private validateRPM(rpm: number): string {
    rpm = Number.parseFloat(rpm.toString());
    if (Number.isNaN(rpm)) {
      return "Input is not a number";
    }
    return "";
  }

  private validatePower(power: number): string {
    power = Number.parseFloat(power.toString());
    if (Number.isNaN(power)) {
      return "Input is not a number";
    } else if (power > 100) {
      return "Power cannot be greater than 100%";
    } else if (power < -100) {
      return "Power cannot be less than -100%";
    }
    return "";
  }

  private validatePosition(position: number): string {
    position = Number.parseFloat(position.toString());
    if (Number.isNaN(position)) {
      return "Input is not a number";
    }
    return "";
  }

  validate(): { [key: string]: string } {
    let toReturn: { [key: string]: string } = {};
    switch (this.movementType) {
      case "Go":
        toReturn = {
          speed: this.validatePower(this.speed),
        };
        break;
      case "Go For":
        toReturn = {
          speed: this.validateRPM(this.speed),
          revolutions: this.validateRevolutions(this.revolutions),
        };
        break;
      case "Go To":
        toReturn = {
          speed: this.validateRPM(this.speed),
          position: this.validatePosition(this.position),
        };
        break;
    }
    return toReturn;
  }

  asObject(): {
    type: string;
    request: any;
  } {
    let req;
    switch (this.movementType) {
      case "Go":
        req = new SetPowerRequest();
        req.setPowerPct((this.power * this.direction) / 100);
        break;
      case "Go For":
        req = new GoForRequest();
        req.setRpm(this.rpm * this.direction);
        req.setRevolutions(this.revolutions);
        break;
      case "Go To":
        req = new GoToRequest();
        req.setRpm(this.rpm);
        req.setPositionRevolutions(this.position);
        break;
    }
    return {
      type: this.type.toString(),
      request: req,
    };
  }
}
</script>

<style scoped></style>