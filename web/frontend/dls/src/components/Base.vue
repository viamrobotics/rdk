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
          :disabled="!baseStatus"
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
          class="border border-black p-2 h-72"
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
                >Discrete</Tab
              >
            </Tabs>
          </div>
          <div
            v-if="selectedItem === 'keyboard'"
            class="border border-black p-4"
            :style="{ maxHeight: maxHeight + 'px' }"
          >
            <div>
              <div>
                <div class="grid grid-cols-2">
                  <div>
                    <div>
                      <div class="flex">
                        <input id="angle" type="hidden" value="45" />
                        <ViamInput
                          type="number"
                          color="primary"
                          group="False"
                          variant="primary"
                          class="pr-2 w-32"
                          inputId="distance"
                          v-model="increment"
                        >
                          <span class="text-xs">Increment (mm)</span>
                        </ViamInput>
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
                      </div>
                    </div>
                    <div class="flex pt-6">
                      <KeyboardInput
                        @arc-right="$emit('arc-right')"
                        @arc-left="$emit('arc-left')"
                        @back-arc-right="$emit('back-arc-right')"
                        @back-arc-left="$emit('back-arc-left')"
                        @forward="$emit('forward')"
                        @backward="$emit('backward')"
                        @spin-clockwise="$emit('spin-clockwise')"
                        @spin-counter-clockwise="
                          $emit('spin-counter-clockwise')
                        "
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
                          <select
                            class="form-select appearance-none block w-full px-3 py-1.5 text-base font-normal text-gray-700 bg-white bg-clip-padding bg-no-repeat border border-solid border-gray-300 rounded transition ease-in-out m-0 focus:text-gray-700 focus:bg-white focus:border-blue-600 focus:outline-none"
                            aria-label="Default select example"
                            @change="$emit('show-base-camera')"
                            v-model="selectedValue"
                          >
                            <option selected value="NoCamera">No Camera</option>
                            <option value="Camera1">Camera1</option>
                          </select>
                          <div
                            class="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2"
                          >
                            <svg
                              class="h-4 w-4 stroke-2"
                              :class="['text-gray-700 dark:text-gray-300']"
                              viewBox="0 0 24 24"
                              stroke="currentColor"
                              stroke-linejoin="round"
                              stroke-linecap="round"
                              fill="none"
                            >
                              <path d="M18 16L12 22L6 16" />
                            </svg>
                          </div>
                        </div>
                      </div>
                    </div>
                    <div
                      class="bg-black w-48 h-48 transition-all duration-300 ease-in-out"
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
            class="border border-black p-4 grid grid-cols-1"
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
              <div class="column" v-if="movementMode === 'Straight'">
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
                class="column"
                v-if="movementMode === 'Spin' || movementMode === 'Arc'"
              >
                <p class="text-xs">Movement Type</p>
                <RadioButtons
                  :options="['Clockwise', 'Counterclockwise']"
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
                  min="0"
                  max="360"
                  unit="<sup class='text-xs'>O</sup>"
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
  increment = 500;
  speed = 300;
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
}
</script>

<style scoped></style>
