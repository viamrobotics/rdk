<template>
  <div>
    <Collapse>
      <div class="flex">
        <h2 class="p-4 text-xl">{{ streamName }}</h2>
        <Breadcrumbs :crumbs="crumbs" disabled="true"></Breadcrumbs>
      </div>
      <template v-slot:content>
        <div
          class="border border-black p-2"
          :style="{ height: maxHeight + 'px' }"
        >
          <Container>
            <div class="pt-4">
              <span class="pr-2">View Camera</span>
              <ViamSwitch
                centered
                name="camera"
                id="camera"
                :option="camera"
                @change="toggleExpand()"
              ></ViamSwitch>
              <div class="float-right pb-4">
                <div class="flex">
                  <div class="w-64" v-if="camera">
                    <p class="mb-1 text-gray-800 font-label dark:text-gray-200">
                      Refresh frequency
                    </p>
                    <div class="relative">
                      <select
                        class="form-select appearance-none block w-full px-3 py-1.5 text-base font-normal text-gray-700 bg-white bg-clip-padding bg-no-repeat border border-solid border-gray-300 rounded transition ease-in-out m-0 focus:text-gray-700 focus:bg-white focus:border-blue-600 focus:outline-none"
                        aria-label="Default select example"
                        v-model="selectedValue"
                        @change="selectCameraView()"
                      >
                        <option value="manual">Manual Refresh</option>
                        <option value="30">Every 30 seconds</option>
                        <option value="10">Every 10 seconds</option>
                        <option value="1">Every second</option>
                        <option value="live">Live</option>
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
                  <div class="pl-2 pr-2 pt-7">
                    <ViamButton
                      v-if="camera"
                      color="black"
                      group
                      variant="primary"
                      @click="refreshCamera()"
                    >
                      <template v-slot:icon>
                        <ViamIcon color="white" :path="mdiRestore"
                          >Refresh</ViamIcon
                        >
                      </template>
                      Refresh
                    </ViamButton>
                  </div>
                  <div class="pr-2 pt-7">
                    <ViamButton
                      v-if="camera"
                      color="primary"
                      group
                      variant="primary"
                      @click="$emit('download-screenshot')"
                    >
                      <template v-slot:icon>
                        <ViamIcon :path="mdiCameraIris">Download</ViamIcon>
                      </template>
                      Export Screenshot
                    </ViamButton>
                  </div>
                </div>
              </div>
              <div
                class="bg-black clear-both h-fit transition-all duration-300 ease-in-out"
                v-if="camera"
                :id="streamId"
              ></div>
            </div>
            <div class="pt-4">
              <span class="pr-2">Point Cloud Data</span>
              <popper
                trigger="clickToOpen"
                :options="{
                  placement: 'bottom',
                  modifiers: { offset: { offset: '0,10px' } },
                }"
              >
                <div class="popper">
                  <ul>
                    <li>When turned on, point cloud will be recalculated</li>
                  </ul>
                </div>

                <button slot="reference">
                  <ViamIcon color="grey" :path="mdiInformationOutline"
                    >Info</ViamIcon
                  >
                </button>
              </popper>
              <ViamSwitch
                centered
                name="pcd"
                id="pcd-button"
                :option="pcd"
                @change="togglePCDExpand()"
              ></ViamSwitch>
              <div v-if="pcd" class="transition-all duration-300 ease-in-out">
                <div class="float-right pb-4">
                  <ViamButton color="black" group variant="primary">
                    <template v-slot:icon>
                      <ViamIcon color="white" :path="mdiRestore"
                        >Refresh</ViamIcon
                      >
                    </template>
                    Refresh
                  </ViamButton>
                  <ViamButton color="primary" group variant="primary">
                    <template v-slot:icon>
                      <ViamIcon :path="mdiImageFilterCenterFocus"
                        >Center</ViamIcon
                      >
                    </template>
                    Center
                  </ViamButton>
                  <ViamButton
                    color="primary"
                    group
                    variant="primary"
                    @click="$emit('download-raw-data')"
                  >
                    <template v-slot:icon>
                      <ViamIcon :path="mdiDownloadOutline">Download</ViamIcon>
                    </template>
                    Download Raw Data
                  </ViamButton>
                </div>
                <div
                  class="bg-black clear-both h-96"
                  id="pcd"
                  @click="pcdClick"
                ></div>
                <div class="float-right">
                  <span class="text-xs">Controls</span>
                  <popper
                    trigger="clickToOpen"
                    :options="{
                      placement: 'bottom',
                      modifiers: { offset: { offset: '0,10px' } },
                    }"
                  >
                    <div class="popper">
                      <ul>
                        <li>Rotate - Left/Click + Drag</li>
                        <li>Pan - Right/Two Finger Click + Drag</li>
                        <li>Zoom - Wheel/Two Finger Scroll</li>
                      </ul>
                    </div>

                    <button slot="reference">
                      <ViamIcon color="grey" :path="mdiInformationOutline"
                        >Info</ViamIcon
                      >
                    </button>
                  </popper>
                </div>
                <div class="grid grid-cols-1 divide-y clear-both">
                  <div>
                    <Container class="pt-4">
                      <div>
                        <h2>Segmentation Settings</h2>
                        <div class="relative">
                          <select
                            class="form-select appearance-none block w-full px-3 py-1.5 text-base font-normal text-gray-700 bg-white bg-clip-padding bg-no-repeat border border-solid border-gray-300 rounded transition ease-in-out m-0 focus:text-gray-700 focus:bg-white focus:border-blue-600 focus:outline-none"
                            aria-label="Select segmenter"
                            @change="changeSegmenter"
                            v-model="selectedValue"
                          >
                            <option value="" selected disabled>Choose</option>
                            <option
                              v-for="segmenter in segmenterNames"
                              v-bind:key="segmenter"
                              :value="segmenter"
                            >
                              {{ segmenter }}
                            </option>
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
                        <div class="row flex">
                          <div
                            class="column flex-auto pr-2 w-1/3"
                            v-for="param in segmenterParameterNames"
                            :key="param.getName()"
                          >
                            <ViamInput
                              color="primary"
                              group="False"
                              variant="primary"
                              class="text-xs"
                              :type="parameterType(param.getType())"
                              :v-model="segmenterParameters[param.getName()]"
                              id="param.getName()"
                              v-model.number="
                                segmenterParameters[param.getName()]
                              "
                              >{{ param.getName() }}</ViamInput
                            >
                          </div>
                        </div>
                      </div>
                    </Container>
                    <div class="p-4 float-right">
                      <ViamButton
                        :loading="pcdObject && pcdObject.calculatingSegments"
                        color="black"
                        group
                        variant="primary"
                        @click="findSegments"
                        >FIND SEGMENTS</ViamButton
                      >
                    </div>
                  </div>
                  <div class="pt-4">
                    <div class="grid grid-cols-2">
                      <div>
                        <div>
                          <span class="text-xs">Selected Point Position</span>
                        </div>
                        <div class="flex">
                          <ViamInput
                            type="number"
                            color="primary"
                            group="False"
                            variant="primary"
                            class="text-xs pr-2 w-32"
                            disabled
                            :value="x"
                            >X
                          </ViamInput>
                          <ViamInput
                            type="number"
                            color="primary"
                            group="False"
                            variant="primary"
                            class="text-xs pr-2 w-32"
                            disabled
                            :value="y"
                            >Y
                          </ViamInput>
                          <ViamInput
                            type="number"
                            color="primary"
                            group="False"
                            variant="primary"
                            class="text-xs pr-2 w-32"
                            disabled
                            :value="z"
                            >Z
                          </ViamInput>
                          <div class="p-4">
                            <ViamButton
                              color="black"
                              group
                              variant="primary"
                              @click="pcdMove"
                              >Move</ViamButton
                            >
                          </div>
                        </div>
                      </div>
                      <div class="grid grid-cols-1">
                        <span class="text-xs">Distance From Camera</span>
                        <span class="pt-4">211mm</span>
                      </div>
                    </div>
                    <div class="flex pt-4 pb-8">
                      <div class="column">
                        <p class="text-xs">Selection Type</p>
                        <RadioButtons
                          :options="['Center Point', 'Bounding Box', 'Cropped']"
                          :disabledOptions="[]"
                          v-on:selectOption="selectObject($event)"
                        />
                      </div>
                      <div class="pl-8">
                        <p class="text-xs">Segmented Objects</p>
                        <select
                          class="block appearance-none w-full border border-gray-300 dark:border-black-700 pr-8 leading-tight focus:outline-none transition-colors duration-150 ease-in-out"
                          :class="[
                            'bg-gray-200 dark:bg-gray-800 text-gray-800 dark:text-gray-200 cursor-not-allowed',
                            'pointer-events-none',
                            'py-2 pl-2',
                          ]"
                          disabled
                        >
                          <option disabled selected value="">Null</option>
                        </select>
                      </div>
                      <div class="pl-8">
                        <div class="grid grid-cols-1">
                          <span class="text-xs">Object Points</span>
                          <span class="pt-2">null</span>
                        </div>
                      </div>
                    </div>
                    <div class="flex pt-4 pb-8">
                      <table
                        class="table-auto border-collapse border border-slate-400"
                      >
                        <tr>
                          <th class="border border-slate-300">Object</th>
                          <th class="border border-slate-300">PCD</th>
                          <th class="border border-slate-300">Point</th>
                          <th class="border border-slate-300">Bounding Box</th>
                        </tr>
                        <tr>
                          <td class="border border-slate-300">Full Image</td>
                          <td class="border border-slate-300">
                            <viam-button group @click="fullImage"
                              >GO</viam-button
                            >
                          </td>
                          <td class="border border-slate-300">--</td>
                        </tr>
                        <tr v-for="(seg, i) in objects" :key="seg">
                          <td class="border border-slate-300">Object ${i}</td>
                          <td class="border border-slate-300">
                            <viam-button group @click="segmentLoad(i)"
                              >GO</viam-button
                            >
                          </td>
                          <td class="border border-slate-300">
                            <viam-button group @click="pointLoad(i)"
                              >GO</viam-button
                            >
                          </td>
                          <td class="border border-slate-300">
                            <viam-button group @click="boundingBoxLoad(i)"
                              >GO</viam-button
                            >
                          </td>
                        </tr>
                      </table>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </Container>
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
import ViamSwitch from "./Switch.vue";
import ViamIcon from "./ViamIcon.vue";
import RadioButtons from "./RadioButtons.vue";
import Popper from "vue-popperjs";
import "vue-popperjs/dist/vue-popper.css";
import {
  mdiRestore,
  mdiImageFilterCenterFocus,
  mdiCameraIris,
  mdiDownloadOutline,
  mdiInformationOutline,
} from "@mdi/js";

@Component({
  components: {
    Collapse,
    Breadcrumbs,
    ViamSwitch,
    Popper,
    ViamIcon,
    RadioButtons,
  },
})
export default class Base extends Vue {
  @Prop({ default: null }) streamName!: string;
  @Prop({ default: null }) crumbs!: [string];
  @Prop({ default: true }) connectedCamera!: boolean;
  @Prop({ default: true }) connectedPCD!: boolean;
  @Prop({ default: 0 }) x?: number;
  @Prop({ default: 0 }) y?: number;
  @Prop({ default: 0 }) z?: number;
  @Prop({ default: null }) pcdObject?: Record<string, unknown>;
  @Prop({ default: null }) segmenterNames?: [string];
  @Prop({ default: null }) segmentAlgo?: string;
  @Prop({ default: null }) segmentObjects?: [string];
  @Prop({ default: null }) segmenterParameterNames?: [string];
  @Prop({ default: null }) parameterType?: [string];
  @Prop({ default: null }) segmenterParameters?: Record<string, unknown>;

  mdiInformationOutline = mdiInformationOutline;
  mdiDownloadOutline = mdiDownloadOutline;
  mdiCameraIris = mdiCameraIris;
  mdiImageFilterCenterFocus = mdiImageFilterCenterFocus;
  mdiRestore = mdiRestore;
  camera = !this.connectedCamera;
  pcd = !this.connectedPCD;
  maxHeight = 150;
  selectedValue = "manual";
  streamId = "stream-" + this.streamName;
  pcdId = "pcd-" + this.streamName;
  selected = "";
  speed = 0;
  min = 0;
  max = 500;

  beforeMount(): void {
    window.addEventListener("resize", this.resizeContent);
  }

  beforeDestroy(): void {
    window.removeEventListener("resize", this.resizeContent);
  }
  mounted(): void {
    this.resizeContent();
  }
  toggleExpand(): void {
    this.camera = !this.camera;
    this.$emit("toggle-camera", this.camera);
    this.resizeContent();
  }

  selectCameraView(): void {
    this.$emit("selected-camera-view", this.selectedValue);
  }

  refreshCamera(): void {
    this.$emit('refresh-camera', 'manual')
  }

  pcdClick(e: Event): void {
    this.$emit("pcd-click", e);
  }

  pcdMove(e: Event): void {
    this.$emit("pcd-move", e);
  }

  changeSegmenter(): void {
    this.$emit("change-segmenter", this.selectedValue);
  }

  findSegments(): void {
    this.$emit("find-segments", this.selectedValue, this.segmenterParameters);
  }

  fullImage(e: Event): void {
    this.$emit("full-image", e);
  }

  selectObject(e: Event): void {
    this.$emit("select-object", e);
  }

  pointLoad(i: number): void {
    this.$emit("point-load", i);
  }

  segmentLoad(i: number): void {
    this.$emit("segment-load", i);
  }

  boundingBoxLoad(i: number): void {
    this.$emit("bounding-box-load", i);
  }

  togglePCDExpand(): void {
    this.pcd = !this.pcd;
    this.$emit("toggle-pcd", this.pcd);
    this.resizeContent();
  }
  resizeContent(): void {
    // TODO: revisit resize logic, remove static numbers
    if (this.camera && this.pcd) {
      this.maxHeight = 2250;
    } else if (this.camera && !this.pcd) {
      this.maxHeight = 1250;
    } else if (this.pcd && !this.camera) {
      this.maxHeight = 1100;
    } else {
      this.maxHeight = 150;
    }
  }
}
</script>

<style scoped></style>
