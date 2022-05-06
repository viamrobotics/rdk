<template>
  <div>
    <Collapse>
      <div class="flex float-left">
        <h2 class="p-4 text-xl">{{ baseName }}</h2>
        <Breadcrumbs :crumbs="crumbs" disabled="true"></Breadcrumbs>
      </div>
      <div class="p-2 float-right">
        <ViamButton
          color="danger"
          group
          variant="primary"
          :disabled="!baseStatus && selectedItem !== 'keyboard'"
          @click="baseStop()"
        >
          <template v-slot:icon>
            <ViamIcon color="white" :path="mdiCloseOctagonOutline"
              >STOP</ViamIcon
            >
          </template>
          STOP
        </ViamButton>
      </div>
      <template v-slot:content>
        <div
          class="border border-t-0 border-black pt-2 pb-4"
          :style="{ maxHeight: maxHeight + 'px' }"
        >
          <div>
            <Tabs>
              <Tab
                :selected="selectedItem === 'keyboard'"
                @select="selectedItem = 'keyboard'"
                @click="$emit('base-change-tab', selectedItem)"
                >Keyboard</Tab
              >
              <Tab
                :selected="selectedItem === 'discrete'"
                @select="selectedItem = 'discrete'"
                @click="resetDiscreteState()"
                >Discrete</Tab
              >
            </Tabs>
          </div>
          <div
            v-if="selectedItem === 'keyboard'"
            class="p-4"
            :style="{ maxHeight: maxHeight + 'px' }"
          >
            <div>
              <div>
                <div class="grid grid-cols-2">
                  <div>
                    <div>
                      <div class="flex">
                        <!-- May need a speed input
                        <ViamInput
                          type="number"
                          color="primary"
                          group="False"
                          variant="primary"
                          class="pr-2 w-32"
                          inputId="speed"
                          v-model="speed"
                        >
                          <span class="text-xs">Speed (mm/sec)</span>
                        </ViamInput>
                        -->
                      </div>
                    </div>
                    <div class="flex pt-6">
                      <KeyboardInput
                        @keyboard-ctl="keyboardCtl"
                      >
                      </KeyboardInput>
                    </div>
                  </div>
                  <div class="flex" v-if="camera">
                    <div class="pr-4">
                      <div class="w-64">
                        <p
                          class="mb-1 text-gray-800 font-label dark:text-gray-200"
                        >
                          Select Camera
                        </p>
                        <div class="relative">
                          <ViamSelect
                            :options="cameraOptions"
                            v-model="selectedValue"
                            @selected="$emit('show-base-camera')"
                          >
                          </ViamSelect>
                        </div>
                      </div>
                    </div>
                    <div
                      class="w-48 h-48 transition-all duration-300 ease-in-out"
                      v-if="selectedValue !== 'NoCamera'"
                      :id="streamId"
                    ></div>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div
            v-if="selectedItem === 'discrete'"
            class="pr-4 pl-4 pt-4 grid grid-cols-1"
            :style="{ maxHeight: maxHeight + 'px' }"
          >
            <div>
              <div class="column">
                <p class="text-xs">Movement Mode</p>
                <RadioButtons
                  :options="['Straight', 'Arc', 'Spin']"
                  defaultOption="Straight"
                  :disabledOptions="[]"
                  v-on:selectOption="setMovementMode($event)"
                />
              </div>
            </div>
            <div class="flex pt-2">
              <div class="column pr-2" v-if="movementMode === 'Straight'">
                <p class="text-xs">Movement Type</p>
                <RadioButtons
                  :options="['Continous', 'Discrete']"
                  defaultOption="Continous"
                  :disabledOptions="[]"
                  v-on:selectOption="setMovementType($event)"
                />
              </div>
              <div
                class="column pr-2"
                v-if="movementMode === 'Straight' || movementMode === 'Arc'"
              >
                <p class="text-xs">Direction</p>
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
                v-model="speed"
                inputId="speed"
                class="text-xs pr-2 w-32"
                >Speed (mm/sec)
              </ViamInput>
              <ViamInput
                v-if="movementMode === 'Straight' || movementMode === 'Arc'"
                type="number"
                color="primary"
                group="False"
                variant="primary"
                v-model="increment"
                inputId="distance"
                :disabled="movementType === 'Continous'"
                class="text-xs pr-2 w-32"
                >Distance (mm)
              </ViamInput>
            </div>
            <div class="flex">
              <div
                class="column pr-2"
                v-if="movementMode === 'Spin' || movementMode === 'Arc'"
              >
                <p class="text-xs">Movement Type</p>
                <RadioButtons
                  :options="['Clockwise', 'Counterclockwise']"
                  defaultOption="Clockwise"
                  :disabledOptions="[]"
                  v-on:selectOption="setSpinType($event)"
                />
              </div>
              <div
                class="column pl-4"
                v-if="movementMode === 'Spin' || movementMode === 'Arc'"
              >
                <Range
                  id="angle"
                  :min="0"
                  :max="360"
                  :step="90"
                  v-model="maxClusteringRadius"
                  unit="°"
                  name="Max Clustering Radius"
                ></Range>
              </div>
            </div>
            <div class="flex flex-row-reverse">
              <div>
                <ViamButton
                  color="success"
                  group
                  variant="primary"
                  :disabled="baseStatus"
                  @click="baseRun()"
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
} from "@mdi/js";
import Tabs from "./Tabs.vue";
import Tab from "./Tab.vue";
import RadioButtons from "./RadioButtons.vue";
import "vue-popperjs/dist/vue-popper.css";
import ViamButton from "./Button.vue";

@Component({
  components: {
    Collapse,
    Breadcrumbs,
    ViamIcon,
    RadioButtons,
    Tabs,
    Tab,
    ViamButton,
  },
})
export default class Base extends Vue {
  @Prop({ default: null }) streamName!: string;
  @Prop({ default: null }) baseName!: string;
  @Prop({ default: null }) crumbs!: [string];
  @Prop({ default: true }) connectedCamera!: boolean;
  @Prop({ default: false }) baseStatus!: boolean;

  mdiRestore = mdiRestore;
  mdiPlayCircleOutline = mdiPlayCircleOutline;
  mdiCloseOctagonOutline = mdiCloseOctagonOutline;

  camera = this.connectedCamera;
  maxHeight = 500;
  selectedValue = "NoCamera";
  isContinuous = true;
  streamId = "stream-preview-" + this.streamName;
  selectedItem = "keyboard";
  pressedKey = 0;
  movementMode = "Straight";
  movementType = "Continous";
  direction = "Forwards";
  spinType = "";
  increment = 1000;
  maxClusteringRadius = 90;

  speed = 200;
  angle = 0;
  cameraOptions = [
    { value: "NoCamera", label: "No Camera" },
    { value: "Camera1", label: "Camera1" },
  ];
  beforeMount(): void {
    window.addEventListener("resize", this.resizeContent);
  }

  beforeDestroy(): void {
    window.removeEventListener("resize", this.resizeContent);
  }

  mounted(): void {
    this.resizeContent();
  }

  resetDiscreteState(): void {
    this.movementMode = "Straight";
    this.movementType = "Continous";
    this.direction = "Forwards";
    this.spinType = "";
  }

  setMovementMode(e: string): void {
    console.log(e);
    this.movementMode = e;
  }
  setMovementType(e: string): void {
    console.log(e);
    this.movementType = e;
  }
  setSpinType(e: string): void {
    console.log(e);
    this.spinType = e;
  }
  setDirection(e: string): void {
    console.log(e);
    this.direction = e;
  }
  baseRun(): void {
    this.$emit(
      "base-run",
      this.movementMode,
      this.movementType,
      this.spinType,
      this.direction
    );
  }
  baseStop(): void {
    this.$emit(
      "base-stop",
      this.movementMode,
      this.movementType,
      this.spinType,
      this.direction
    );
  }
  resizeContent(): void {
    if (this.camera) {
      this.maxHeight = 1500;
    } else {
      this.maxHeight = 500;
    }
  }
  keyboardCtl(keysPressed: any): void {
    let toEmit = {
      baseName: this.baseName,
      forward: keysPressed.forward,
      backward: keysPressed.backward,
      right: keysPressed.right,
      left: keysPressed.left,
    };
    this.$emit("keyboard-ctl", toEmit);
  }
}
</script>

<style scoped></style>
