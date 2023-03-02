
<script setup lang="ts">

import { nextTick } from 'vue';
import { grpc } from '@improbable-eng/grpc-web';
import { Client, commonApi, slamApi } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';
import PCD from './pcd/pcd-view.vue';
import Slam2dRender from './slam-2d-render.vue';

interface Props {
  name: string
  resources: commonApi.ResourceName.AsObject[]
  client: Client
}

const props = defineProps<Props>();

const selected2dValue = $ref('manual');
const selected3dValue = $ref('manual');
let pointCloudUpdateCount = $ref(0);
let pointcloud = $ref<Uint8Array | undefined>();
let pose = $ref<commonApi.Pose | undefined>();
let show2d = $ref(false);
let show3d = $ref(false);

const loaded2d = $computed(() => (pointcloud !== undefined && pose !== undefined));

let slam2dIntervalId = -1;
let slam3dIntervalId = -1;

const fetchSLAMMap = (name: string) => {
  return nextTick(() => {
    const req = new slamApi.GetMapRequest();
    req.setName(name);
    req.setMimeType('pointcloud/pcd');

    rcLogConditionally(req);
    props.client.slamService.getMap(req, new grpc.Metadata(), (error, response) => {
      if (error) {
        return displayError(error);
      }

      /*
       * TODO: This is a hack workaround the fac that at time of writing response!.getPointCloud()!.getPointCloud_asU8()
       * appears to return the binary data of the entire response, not response.pointcloud.pointcloud.
       * Tracked in ticket: https://viam.atlassian.net/browse/RSDK-2108
       */

      const base64DecodedPointCloudString = atob(response!.toObject().pointCloud!.pointCloud! as string);
      pointcloud = Uint8Array.from(base64DecodedPointCloudString, (char: string): number => char.codePointAt(0)!);
      // END NOTE
      pointCloudUpdateCount += 1;
    });
  });
};

const fetchSLAMPose = (name: string) => {
  return nextTick(() => {
    const req = new slamApi.GetPositionRequest();
    req.setName(name);
    props.client.slamService.getPosition(req, new grpc.Metadata(), (error, res): void => {
      if (error) {
        displayError(error);
        return;
      }
      pose = res!.getPose()!.getPose()!;
    });
  });
};

const refresh2d = async (name: string) => {
  const mapPromise = fetchSLAMMap(name);
  const posePromise = fetchSLAMPose(name);
  await mapPromise;
  await posePromise;
};

// eslint-disable-next-line require-await
const updateSLAM2dRefreshFrequency = async (name: string, time: 'manual' | 'off' | string) => {
  window.clearInterval(slam2dIntervalId);

  if (time === 'manual') {
    refresh2d(name);

  } else if (time === 'off') {
    // do nothing
  } else {

    refresh2d(name);
    slam2dIntervalId = window.setInterval(() => {
      refresh2d(name);
    }, Number.parseFloat(time) * 1000);
  }
};

const updateSLAM3dRefreshFrequency = (name: string, time: 'manual' | 'off' | string) => {
  clearInterval(slam3dIntervalId);

  if (time === 'manual') {
    fetchSLAMMap(name);
  } else if (time === 'off') {
    // do nothing
  } else {
    fetchSLAMMap(name);
    slam3dIntervalId = window.setInterval(() => {
      fetchSLAMMap(name);
    }, Number.parseFloat(time) * 1000);
  }
};

const toggle2dExpand = () => {
  show2d = !show2d;
  updateSLAM2dRefreshFrequency(props.name, show2d ? selected2dValue : 'off');
};

const toggle3dExpand = () => {
  show3d = !show3d;
  updateSLAM3dRefreshFrequency(props.name, show3d ? selected3dValue : 'off');
};

const selectSLAM2dRefreshFrequency = () => {
  updateSLAM2dRefreshFrequency(props.name, selected2dValue);
};

const selectSLAMPCDRefreshFrequency = () => {
  updateSLAM3dRefreshFrequency(props.name, selected3dValue);
};

// eslint-disable-next-line require-await
const refresh2dMap = async () => {
  updateSLAM2dRefreshFrequency(props.name, selected2dValue);
};

const refresh3dMap = () => {
  updateSLAM3dRefreshFrequency(props.name, selected3dValue);
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
              :value="show2d ? 'on' : 'off'"
              @input="toggle2dExpand()"
            />
            <span class="pr-2">View SLAM Map (2D)</span>
          </div>
          <div class="float-right pb-4">
            <div class="flex">
              <div
                v-if="show2d"
                class="w-64"
              >
                <p class="font-label mb-1 text-gray-800">
                  Refresh frequency
                </p>
                <div class="relative">
                  <select
                    v-model="selected2dValue"
                    class="
                      m-0 w-full appearance-none border border-solid border-black bg-white bg-clip-padding px-3 py-1.5
                      text-xs font-normal text-gray-700 focus:outline-none
                    "
                    aria-label="Default select example"
                    @change="selectSLAM2dRefreshFrequency()"
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
                    <option value="1">
                      Every second
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
                  v-if="show2d"
                  icon="refresh"
                  label="Refresh"
                  @click="refresh2dMap()"
                />
              </div>
            </div>
          </div>
          <Slam2dRender
            v-if="loaded2d && show2d"
            :point-cloud-update-count="pointCloudUpdateCount"
            :pointcloud="pointcloud"
            :pose="pose"
            :name="name"
            :resources="resources"
            :client="client"
          />
        </div>
        <div class="pt-4">
          <div class="flex items-center gap-2">
            <v-switch
              :value="show3d ? 'on' : 'off'"
              @input="toggle3dExpand()"
            />
            <span class="pr-2">View SLAM Map (3D))</span>
          </div>
          <div class="float-right pb-4">
            <div class="flex">
              <div
                v-if="show3d"
                class="w-64"
              >
                <p class="font-label mb-1 text-gray-800">
                  Refresh frequency
                </p>
                <div class="relative">
                  <select
                    v-model="selected3dValue"
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
                    <option value="1">
                      Every second
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
                  v-if="show3d"
                  icon="refresh"
                  label="Refresh"
                  @click="refresh3dMap()"
                />
              </div>
            </div>
          </div>
          <PCD
            v-if="show3d"
            :resources="resources"
            :pointcloud="pointcloud"
            :client="client"
          />
        </div>
      </div>
    </div>
  </v-collapse>
</template>
