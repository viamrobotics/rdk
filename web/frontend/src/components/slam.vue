
<script setup lang="ts">

import { $ref, $computed } from 'vue/macros';
import { grpc } from '@improbable-eng/grpc-web';
import { Client, commonApi, ResponseStream, ServiceError, slamApi } from '@viamrobotics/sdk';
import { displayError, isServiceError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';
import PCD from './pcd/pcd-view.vue';
import Slam2dRender from './slam-2d-render.vue';

type MapAndPose = { map: Uint8Array, pose: commonApi.Pose}

const props = defineProps<{
  name: string
  resources: commonApi.ResourceName.AsObject[]
  client: Client
}>();

const selected2dValue = $ref('manual');
const selected3dValue = $ref('manual');
let pointCloudUpdateCount = $ref(0);
let pointcloud = $ref<Uint8Array | undefined>();
let pose = $ref<commonApi.Pose | undefined>();
let show2d = $ref(false);
let show3d = $ref(false);
let refresh2DCancelled = true;
let refresh3DCancelled = true;

const loaded2d = $computed(() => (pointcloud !== undefined && pose !== undefined));

let slam2dTimeoutId = -1;
let slam3dTimeoutId = -1;

const concatArrayU8 = (arrays: Uint8Array[]) => {
  const totalLength = arrays.reduce((acc, value) => acc + value.length, 0);
  const result = new Uint8Array(totalLength);
  let length = 0;
  for (const array of arrays) {
    result.set(array, length);
    length += array.length;
  }
  return result;
};

const fetchSLAMMap = (name: string): Promise<Uint8Array> => {
  return new Promise((resolve, reject) => {
    const req = new slamApi.GetPointCloudMapRequest();
    req.setName(name);
    rcLogConditionally(req);
    const chunks: Uint8Array[] = [];

    const getPointCloudMap: ResponseStream<slamApi.GetPointCloudMapResponse> =
      props.client.slamService.getPointCloudMap(req);
    getPointCloudMap.on('data', (res: { getPointCloudPcdChunk_asU8(): Uint8Array }) => {
      const chunk = res.getPointCloudPcdChunk_asU8();
      chunks.push(chunk);
    });
    getPointCloudMap.on('status', (status: { code: number, details: string, metadata: string }) => {
      if (status.code !== 0) {
        const error = {
          message: status.details,
          code: status.code,
          metadata: status.metadata,
        };
        reject(error);
      }
    });
    getPointCloudMap.on('end', (end: { code: number }) => {
      if (end === undefined || end.code !== 0) {
        // the error will be logged in the 'status' callback
        return;
      }
      const arr = concatArrayU8(chunks);
      resolve(arr);
    });
  });
};

const fetchSLAMPose = (name: string): Promise<commonApi.Pose> => {
  return new Promise((resolve, reject): void => {
    const req = new slamApi.GetPositionRequest();
    req.setName(name);
    props.client.slamService.getPosition(
      req,
      new grpc.Metadata(),
      (error: ServiceError, res: slamApi.GetPositionResponse): void => {
        if (error) {
          reject(error);
          return;
        }
        resolve(res!.getPose()!);
      }
    );
  });
};

const refresh2d = async (name: string) => {
  const map = await fetchSLAMMap(name);
  const returnedPose = await fetchSLAMPose(name);
  const mapAndPose: MapAndPose = {
    map,
    pose: returnedPose,
  };
  return mapAndPose;
};

const handleRefresh2dResponse = (response: MapAndPose): void => {
  pointcloud = response.map;
  pose = response.pose;
  pointCloudUpdateCount += 1;
};

const handleRefresh3dResponse = (response: Uint8Array): void => {
  pointcloud = response;
  pointCloudUpdateCount += 1;
};

const handleError = (errorLocation: string, error: unknown): void => {
  if (isServiceError(error)) {
    displayError(error as ServiceError);
  } else {
    displayError(`${errorLocation} hit error: ${error}`);
  }
};

const scheduleRefresh2d = (name: string, time: string) => {
  const timeoutCallback = async () => {
    try {
      const res = await refresh2d(name);
      handleRefresh2dResponse(res);
    } catch (error) {
      handleError('refresh2d', error);
      return;
    }
    if (refresh2DCancelled) {
      return;
    }
    scheduleRefresh2d(name, time);
  };
  slam2dTimeoutId = window.setTimeout(timeoutCallback, Number.parseFloat(time) * 1000);
};

const scheduleRefresh3d = (name: string, time: string) => {
  const timeoutCallback = async () => {
    try {
      const res = await fetchSLAMMap(name);
      handleRefresh3dResponse(res);
    } catch (error) {
      handleError('fetchSLAMMap', error);
      return;
    }
    if (refresh3DCancelled) {
      return;
    }
    scheduleRefresh3d(name, time);
  };
  slam3dTimeoutId = window.setTimeout(timeoutCallback, Number.parseFloat(time) * 1000);
};

const updateSLAM2dRefreshFrequency = async (name: string, time: 'manual' | 'off' | string) => {
  refresh2DCancelled = true;
  window.clearTimeout(slam2dTimeoutId);

  if (time === 'manual') {
    try {
      const res = await refresh2d(name);
      handleRefresh2dResponse(res);
    } catch (error) {
      handleError('refresh2d', error);
    }
  } else if (time === 'off') {
    // do nothing
  } else {
    refresh2DCancelled = false;
    scheduleRefresh2d(name, time);
  }
};

const updateSLAM3dRefreshFrequency = async (name: string, time: 'manual' | 'off' | string) => {
  refresh3DCancelled = true;
  window.clearTimeout(slam3dTimeoutId);

  if (time === 'manual') {
    try {
      const res = await fetchSLAMMap(name);
      handleRefresh3dResponse(res);
    } catch (error) {
      handleError('fetchSLAMMap', error);
    }
  } else if (time === 'off') {
    // do nothing
  } else {
    refresh3DCancelled = false;
    scheduleRefresh3d(name, time);
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

const refresh2dMap = () => {
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
