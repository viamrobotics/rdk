<template>
  <div>
    <Collapse>
      <div class="flex float-left">
        <h2 class="p-4 text-xl">{{ motorName }}</h2>
        <Breadcrumbs :crumbs="crumbs" disabled="true"></Breadcrumbs>
        <div
          class="p-4 flex items-center flex-wrap"
          v-if="motorStatus.positionReporting"
        >
          <p
            class="flex items-center border border-black rounded-full px-2 leading-tight"
          >
            Position {{ motorStatus.position }}
          </p>
        </div>
        <div class="p-4 flex items-center flex-wrap">
          <ViamBadge color="green" v-if="motorStatus.isPowered"
            >Running</ViamBadge
          >
          <ViamBadge color="gray" v-if="!motorStatus.isPowered">Idle</ViamBadge>
        </div>
      </div>
      <div class="p-2 float-right">
        <ViamButton color="danger" group variant="primary" @click="motorStop">
          <template v-slot:icon>
            <ViamIcon color="white" :path="mdiCloseOctagonOutline"
              >STOP</ViamIcon
            >
          </template>
          STOP
        </ViamButton>
      </div>
      <template v-slot:content>
        <div class="" :style="{ height: maxHeight + 'px' }">
          <div
            class="border border-black p-4 grid grid-cols-1"
            :style="{ maxHeight: maxHeight + 'px' }"
          >
            <div class="grid">
              <div class="column">
                <p class="text-xs pb-2">Set Power</p>
                <RadioButtons
                  :options="
                    motorStatus.positionReporting
                      ? ['Go', 'Go For', 'Go To']
                      : ['Go']
                  "
                  defaultOption="Go"
                  :disabledOptions="[]"
                  v-on:selectOption="setMovementType($event)"
                />
              </div>
              <div class="flex pt-4" v-if="movementType === 'Go To'">
                <div class="place-self-end pr-2">
                  <span class="text-2xl">{{ movementType }}</span>
                  <viam-info-button
                    class="pb-2"
                    :iconPath="mdiInformation"
                    :infoRows="['Relative to Home']"
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
              <div class="flex pt-4" v-if="movementType === 'Go For'">
                <div class="place-self-end pr-2">
                  <span class="text-2xl">{{ movementType }}</span>
                  <viam-info-button
                    class="pb-2"
                    :iconPath="mdiInformation"
                    :infoRows="['Relative to where the robot is currently']"
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
              <div class="flex items-start pt-4" v-if="movementType === 'Go'">
                <div class="place-self-end pr-2">
                  <span class="text-2xl">{{ movementType }}</span>
                  <viam-info-button
                    class="pb-2"
                    :iconPath="mdiInformation"
                    :infoRows="['Continuously moves']"
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
                  :step="1"
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
                    <ViamIcon color="white" :path="mdiPlayCircleOutline"
                      >RUN</ViamIcon
                    >
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
  mdiAlertOctagonOutline,
} from "@mdi/js";
import Tabs from "./Tabs.vue";
import Tab from "./Tab.vue";
import RadioButtons from "./RadioButtons.vue";
import ViamBadge from "./Badge.vue";
import Popper from "vue-popperjs";
import "vue-popperjs/dist/vue-popper.css";
import ViamButton from "./Button.vue";
import { Status } from "proto/api/component/motor/v1/motor_pb";

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
export default class MotorDetail extends Vue {
  @Prop({ default: null }) motorName!: string;
  @Prop({ default: null }) crumbs!: [string];
  @Prop() motorStatus!: Status.AsObject;

  mdiRestore = mdiRestore;
  mdiPlayCircleOutline = mdiPlayCircleOutline;
  mdiCloseOctagonOutline = mdiCloseOctagonOutline;
  mdiInformation = mdiAlertOctagonOutline;
  maxHeight = 500;
  movementType = "Go";
  direction: -1 | 1 = 1;
  position = 0;
  rpm = 0;
  power = 25;
  type = "go";
  speed = 0;
  revolutions = 0;

  beforeMount(): void {
    window.addEventListener("resize", this.resizeContent);
  }

  beforeDestroy(): void {
    window.removeEventListener("resize", this.resizeContent);
  }

  mounted(): void {
    this.resizeContent();
  }

  setMovementType(e: string): void {
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

  setDirection(e: string): void {
    switch (e) {
      case "Forwards":
        this.direction = 1;
        break;
      case "Backwards":
        this.direction = -1;
        break;
      default:
        this.direction = 1;
    }
  }

  motorRun(): void {
    const command = {
      direction: this.direction,
      position: this.position,
      rpm: this.rpm,
      power: this.power,
      type: this.type,
      speed: this.speed,
      revolutions: this.revolutions,
    };
    this.$emit("motor-run", command);
  }

  motorStop(e: Event): void {
    e.preventDefault();
    e.stopPropagation();
    this.$emit("motor-stop");
  }

  resizeContent(): void {
    this.maxHeight = 250;
  }
}
</script>

<style scoped></style>
