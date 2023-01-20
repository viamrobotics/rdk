
<script setup lang="ts">

import { nextTick } from 'vue';
import { grpc } from '@improbable-eng/grpc-web';
import { Client, commonApi, slamApi } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';
import PCD from './pcd.vue';

interface Props {
  name: string
  resources: commonApi.ResourceName.AsObject[]
  client: Client
}

const props = defineProps<Props>();

let showImage = $ref(false);
let showPCD = $ref(false);
const selectedImageValue = $ref('manual');
const selectedPCDValue = $ref('manual');
let imageMap = $ref('');
let pointcloud = $ref<Uint8Array | undefined>();

let slamImageIntervalId = -1;
let slamPCDIntervalId = -1;

const viewSLAMPCDMap = (name: string) => {
  nextTick(() => {
    const req = new slamApi.GetMapRequest();
    req.setName(name);
    req.setMimeType('pointcloud/pcd');

    rcLogConditionally(req);
    props.client.slamService.getMap(req, new grpc.Metadata(), (error, response) => {
      if (error) {
        return displayError(error);
      }
      const pcObject = response!.getPointCloud()!;
      pointcloud = pcObject.getPointCloud_asU8();
    });
  });
};

const viewSLAMImageMap = (name: string) => {
  const req = new slamApi.GetMapRequest();
  req.setName(name);
  req.setMimeType('image/jpeg');
  req.setIncludeRobotMarker(true);

  rcLogConditionally(req);
  props.client.slamService.getMap(req, new grpc.Metadata(), (error, response) => {
    if (error) {
      return displayError(error);
    }
    const blob = new Blob([response!.getImage_asU8()], { type: 'image/jpeg' });
    imageMap = URL.createObjectURL(blob);
  });
};

const updateSLAMImageRefreshFrequency = (name: string, time: 'manual' | 'off' | string) => {
  window.clearInterval(slamImageIntervalId);

  if (time === 'manual') {
    viewSLAMImageMap(name);
  } else if (time === 'off') {
    // do nothing
  } else {
    viewSLAMImageMap(name);
    slamImageIntervalId = window.setInterval(() => {
      viewSLAMImageMap(name);
    }, Number.parseFloat(time) * 1000);
  }
};

const updateSLAMPCDRefreshFrequency = (name: string, time: 'manual' | 'off' | string) => {
  clearInterval(slamPCDIntervalId);

  if (time === 'manual') {
    viewSLAMPCDMap(name);
  } else if (time === 'off') {
    // do nothing
  } else {
    viewSLAMPCDMap(name);
    slamPCDIntervalId = window.setInterval(() => {
      viewSLAMPCDMap(name);
    }, Number.parseFloat(time) * 1000);
  }
};

const toggleImageExpand = () => {
  showImage = !showImage;
  updateSLAMImageRefreshFrequency(props.name, showImage ? selectedImageValue : 'off');
};

const togglePCDExpand = () => {
  showPCD = !showPCD;
  updateSLAMPCDRefreshFrequency(props.name, showPCD ? selectedPCDValue : 'off');
};

const selectSLAMImageRefreshFrequency = () => {
  updateSLAMImageRefreshFrequency(props.name, selectedImageValue);
};

const selectSLAMPCDRefreshFrequency = () => {
  updateSLAMPCDRefreshFrequency(props.name, selectedPCDValue);
};

const refreshImageMap = () => {
  updateSLAMImageRefreshFrequency(props.name, selectedImageValue);
};

const refreshPCDMap = () => {
  updateSLAMPCDRefreshFrequency(props.name, selectedPCDValue);
};

</script>

<template>
  <v-collapse
    :title="props.name"
    class="slam"
  >
    <v-breadcrumbs
      slot="title"
      crumbs="slam"
    />
    <div class="h-auto border-x border-b border-black p-2">
      <div class="container mx-auto">
        <div class="pt-4">
          <div class="flex items-center gap-2">
            <v-switch
              id="showImage"
              :value="showImage ? 'on' : 'off'"
              @input="toggleImageExpand()"
            />
            <span class="pr-2">View SLAM Map (JPEG)</span>
          </div>
          <div class="float-right pb-4">
            <div class="flex">
              <div
                v-if="showImage"
                class="w-64"
              >
                <p class="font-label mb-1 text-gray-800">
                  Refresh frequency
                </p>
                <div class="relative">
                  <select
                    v-model="selectedImageValue"
                    class="
                      m-0 w-full appearance-none border border-solid border-black bg-white bg-clip-padding px-3 py-1.5
                      text-xs font-normal text-gray-700 focus:outline-none
                    "
                    aria-label="Default select example"
                    @change="selectSLAMImageRefreshFrequency()"
                  >
                    <option value="manual">
                      Manual Refresh
                    </option>
                    <option value="30">
                      Every 30 seconds
                    </option>
                    <option value="10">
                      Every 10 seconds
                    </option>
                    <option value="5">
                      Every 5 seconds
                    </option>
                  </select>
                  <div
                    class="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2"
                  >
                    <svg
                      class="h-4 w-4 stroke-2 text-gray-700"
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
              <div class="px-2 pt-7">
                <v-button
                  v-if="showImage"
                  icon="refresh"
                  label="Refresh"
                  @click="refreshImageMap()"
                />
              </div>
            </div>
          </div>
          <img
            v-if="showImage"
            :src="imageMap"
            width="500"
            height="500"
          >
        </div>
        <div class="pt-4">
          <div class="flex items-center gap-2">
            <v-switch
              :value="showPCD ? 'on' : 'off'"
              @input="togglePCDExpand()"
            />
            <span class="pr-2">View SLAM Map (PCD)</span>
          </div>
          <div class="float-right pb-4">
            <div class="flex">
              <div
                v-if="showPCD"
                class="w-64"
              >
                <p class="font-label mb-1 text-gray-800">
                  Refresh frequency
                </p>
                <div class="relative">
                  <select
                    v-model="selectedPCDValue"
                    class="
                      m-0 w-full appearance-none border border-solid border-black bg-white bg-clip-padding px-3 py-1.5
                      text-xs font-normal text-gray-700 focus:outline-none
                    "
                    aria-label="Default select example"
                    @change="selectSLAMPCDRefreshFrequency()"
                  >
                    <option value="manual">
                      Manual Refresh
                    </option>
                    <option value="30">
                      Every 30 seconds
                    </option>
                    <option value="10">
                      Every 10 seconds
                    </option>
                    <option value="5">
                      Every 5 seconds
                    </option>
                  </select>
                  <div
                    class="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2"
                  >
                    <svg
                      class="h-4 w-4 stroke-2 text-gray-700"
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
              <div class="px-2 pt-7">
                <v-button
                  v-if="showPCD"
                  icon="refresh"
                  label="Refresh"
                  @click="refreshPCDMap"
                />
              </div>
            </div>
          </div>
          <PCD
            v-if="showPCD"
            :resources="resources"
            :pointcloud="pointcloud"
            :client="client"
          />
        </div>
      </div>
    </div>
  </v-collapse>
</template>
