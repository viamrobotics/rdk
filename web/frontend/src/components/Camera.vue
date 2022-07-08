<script lang="ts">

import { ref } from 'vue'
import InfoButton from "./info-button.vue";

interface Props {
  streamName: string
  crumbs: string[]
  connectedCamera: boolean
  connectedPCD: boolean
  x: number
  y: number
  z: number
  pcdObject?: Record<string, unknown>
  segmenterNames?: string[]
  segmentObjects?: Record<string, unknown>[]
  segmenterParameterNames?: string[]
  parameterType?: (type: string) => string
  segmenterParameters?: Record<string, unknown>
  findStatus?: boolean
}

interface Emits {
  (event: 'toggle-camera', camera: boolean): void
  (event: "selected-camera-view", value: string): void
  (event: "refresh-camera", value: string): void
  (event: "pcd-click", event: Event): void
  (event: "full-image", event: Event): void
  (event: "change-segmenter", value: string): void
  (event: "find-segments", value: string, params: Record<string, unknown>): void
  (event: "center-pcd", event: Event): void
  (event: "select-object", event: string, object: string): void
  (event: "point-load", index: number): void
  (event: "segment-load", index: number): void
  (event: "bounding-box-load", index: number): void
  (event: "toggle-pcd", pcd: boolean): void
}

const props = withDefaults(defineProps<Props>(), {
  connectedCamera: false,
  connectedPCD: false,
  x: 0,
  y: 0,
  z: 0,
  findStatus: false
})

const emit = defineEmits<Emits>()

const infoControls = [
  "Rotate - Left/Click + Drag",
  "Pan - Right/Two Finger Click + Drag",
  "Zoom - Wheel/Two Finger Scroll",
];

const camera = ref(props.connectedCamera);
const pcd = ref(props.connectedPCD);
const selectedValue = ref("live");
const selectedSegmenterValue = ref("");
const selectedObject = ref("");

const toggleExpand = () => {
  camera.value = !camera.value;
  emit("toggle-camera", camera.value);
}

const distanceFromCamera = () => {
  return (
    Math.round(
      Math.sqrt(
        Math.pow(props.x, 2) + Math.pow(props.y, 2) + Math.pow(props.z, 2)
      )
    ) || 0
  );
}

const selectCameraView = () => {
  emit("selected-camera-view", selectedValue.value);
}

const refreshCamera = () => {
  emit("refresh-camera", selectedValue.value);
}

const pcdMove = (e: Event) => {
  emit("pcd-move", e);
}

const changeSegmenter = () => {
  emit("change-segmenter", selectedSegmenterValue.value);
}

const findSegments = () => {
  if (props.pcdObject) {
    props.pcdObject.calculatingSegments = true;
  }
  
  emit(
    "find-segments",
    selectedSegmenterValue.value,
    props.segmenterParameters
  );
}

const fullImage = (event: Event) => {
  emit("full-image", event);
}

const centerPCD = (event: Event) => {
  emit("center-pcd", event);
}

const selectObject = (event: string) => {
  emit("select-object", event, selectedObject.value);
}

const changeObject = (event: string) => {
  emit("select-object", event, "Center Point");
}

const pointLoad = (index: number) => {
  emit("point-load", index);
}

const segmentLoad = (index: number) => {
  emit("segment-load", index);
}

const boundingBoxLoad = (index: number) => {
  emit("bounding-box-load", index);
}

const togglePCDExpand = () => {
  pcd.value = !pcd.value;
  emit("toggle-pcd", pcd.value);
}

</script>

<template>
  <v-collapse :title="streamName">
    <v-breadcrumbs slot="header" :crumbs="crumbs.join(',')" />
    <div class="h-auto border-l border-r border-b border-black p-2">
      <div class="container mx-auto">
        <div class="pt-4">
          <span class="pr-2">View Camera</span>
          <v-switch
            id="camera"
            :value="camera ? 'on' : 'off'"
            @input="toggleExpand()"
          />
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
                      class="h-4 w-4 stroke-2 text-gray-700 dark:text-gray-300"
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
                <v-button
                  v-if="camera"
                  icon="refresh"
                  label="Refresh"
                  @click="refreshCamera()"
                />
              </div>
              <div class="pr-2 pt-7">
                <v-button
                  v-if="camera"
                  icon="camera"
                  label="Export Screenshot"
                  @click="$emit('download-screenshot')"
                />
              </div>
            </div>
          </div>
          <div
            v-if="camera"
            :id="`stream-${props.streamName}`"
            class="clear-both h-fit transition-all duration-300 ease-in-out"
          />
        </div>
        <div class="pt-4">
          <span class="pr-2">Point Cloud Data</span>
          <InfoButton :infoRows="['When turned on, point cloud will be recalculated']" />
          <v-switch
            :value="pcd ? 'on' : 'off'"
            @input="togglePCDExpand()"
          />
          <div v-if="pcd" class="transition-all duration-300 ease-in-out">
            <div class="float-right pb-4">
              <v-button
                icon="refresh"
                label="Refresh"
                @click="fullImage"
              />
              <v-button
                icon="center"
                label="Center"
                @click="centerPCD"
              />
              <v-button
                icon="download"
                label="Download Raw Data"
                @click="$emit('download-raw-data')"
              />
            </div>
            <div class="table relative pb-6" id="pcd" @click="emit('pcd-click', $event)">
              <div class="absolute r-0 bottom-0 right-0 whitespace-nowrap">
                <span class="text-xs">Controls</span>
                <InfoButton
                  :infoRows="[
                    'Rotate - Left/Click + Drag',
                    'Pan - Right/Two Finger Click + Drag',
                    'Zoom - Wheel/Two Finger Scroll',
                  ]"
                />
              </div>
            </div>
            <div class="grid grid-cols-1 divide-y clear-both">
              <div>
                <div class="container mx-auto pt-4">
                  <div>
                    <h2>Segmentation Settings</h2>
                    <div class="relative">
                      <select
                        class="form-select appearance-none block w-full px-3 py-1.5 text-base font-normal text-gray-700 bg-white bg-clip-padding bg-no-repeat border border-solid border-gray-300 rounded transition ease-in-out m-0 focus:text-gray-700 focus:bg-white focus:border-blue-600 focus:outline-none"
                        aria-label="Select segmenter"
                        @change="changeSegmenter"
                        v-model="selectedSegmenterValue"
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
                          class="h-4 w-4 stroke-2 text-gray-700 dark:text-gray-300"
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
                </div>
                <div class="p-4 float-right">
                  <v-button
                    :loading="findStatus"
                    :disabled="selectedSegmenterValue === ''"
                    label="FIND SEGMENTS"
                    @click="findSegments"
                  />
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
                        <v-button
                          label="Move"
                          @click="pcdMove"
                        />
                      </div>
                    </div>
                  </div>
                  <div class="grid grid-cols-1">
                    <span class="text-xs">Distance From Camera</span>
                    <span class="pt-4">{{ distanceFromCamera() }}mm</span>
                  </div>
                </div>
                <div class="flex pt-4 pb-8">
                  <div class="column">
                    <p class="text-xs">Selection Type</p>
                    <v-radio
                      options="Center Point, Bounding Box, Cropped"
                      @input="selectObject($event.detail.selected)"
                    />
                  </div>
                  <div class="pl-8">
                    <p class="text-xs">Segmented Objects</p>
                    <select
                      class="block appearance-none w-full border border-gray-300 dark:border-black-700 pr-8 leading-tight focus:outline-none transition-colors duration-150 ease-in-out"
                      :class="['py-2 pl-2']"
                      v-model="selectedObject"
                      @change="changeObject"
                    >
                      <option disabled selected value="">
                        Select Object
                      </option>
                      <option
                        v-for="(seg, i) in segmentObjects"
                        :key="seg[0]"
                        :value="i"
                      >
                        Object {{ i }}
                      </option>
                    </select>
                  </div>
                  <div class="pl-8">
                    <div class="grid grid-cols-1">
                      <span class="text-xs">Object Points</span>
                      <span class="pt-2">{{
                        segmentObjects ? segmentObjects.length : "null"
                      }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </v-collapse>
</template>
