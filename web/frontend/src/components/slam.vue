
<script setup lang="ts">

import { $ref, $computed } from 'vue/macros';
import { grpc } from '@improbable-eng/grpc-web';
import { toast } from '../lib/toast';
import { Struct } from 'google-protobuf/google/protobuf/struct_pb';
import * as THREE from 'three';
import { Client, commonApi, ResponseStream, robotApi, ServiceError, slamApi, motionApi } from '@viamrobotics/sdk';
import { displayError, isServiceError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';
import PCD from './pcd/pcd-view.vue';
import { copyToClipboardWithToast } from '../lib/copy-to-clipboard';
import Slam2dRender from './slam-2d-render.vue';
import { filterResources } from '../lib/resource';
import { onMounted, onUnmounted } from 'vue';

type MapAndPose = { map: Uint8Array, pose: commonApi.Pose}

const props = defineProps<{
  name: string
  resources: commonApi.ResourceName.AsObject[]
  client: Client
  statusStream: ResponseStream<robotApi.StreamStatusResponse> | null
}>();
const refreshErrorMessage = 'Error refreshing map. The map shown may be stale.';
let refreshErrorMessage2d = $ref<string | null>();
let refreshErrorMessage3d = $ref<string | null>();
let selected2dValue = $ref('manual');
let selected3dValue = $ref('manual');
let pointCloudUpdateCount = $ref(0);
let pointcloud = $ref<Uint8Array | undefined>();
let pose = $ref<commonApi.Pose | undefined>();
let show2d = $ref(false);
let show3d = $ref(false);
let showAxes = $ref(true);
let refresh2DCancelled = true;
let refresh3DCancelled = true;
let updatedDest = $ref(false);
let destinationMarker = $ref(new THREE.Vector3());
let moveClick = $ref(true);
const basePose = new commonApi.Pose();
const motionServiceReq = new motionApi.MoveOnMapRequest();

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
    getPointCloudMap.on('status', (status: { code: number, details: string, metadata: grpc.Metadata }) => {
      if (status.code !== 0) {
        const error = {
          message: status.details,
          code: status.code,
          metadata: status.metadata,
        };
        reject(error);
      }
    });
    getPointCloudMap.on('end', (end?: { code: number, details: string, metadata: grpc.Metadata }) => {
      if (end === undefined) {
        const error = { message: 'Stream ended without status code' };
        reject(error);
      } else if (end.code !== 0) {
        const error = {
          message: end.details,
          code: end.code,
          metadata: end.metadata,
        };
        reject(error);
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
      (error: ServiceError | null, res: slamApi.GetPositionResponse | null): void => {
        if (error) {
          reject(error);
          return;
        }
        resolve(res!.getPose()!);
      }
    );
  });
};

const executeMoveOnMap = async () => {
  // get base resources
  const baseResources = filterResources(props.resources, 'rdk', 'component', 'base');
  if (baseResources === undefined) {
    toast.error('No base component detected.');
    return;
  }

  moveClick = !moveClick;

  /*
   * set request name
   * here we set the name of the motion service the user is using
   */
  motionServiceReq.setName('builtin');

  // set pose in frame
  const destination = new commonApi.Pose();
  const value = await fetchSLAMPose(props.name);
  destination.setX(destinationMarker.x);
  destination.setY(destinationMarker.y);
  destination.setZ(destinationMarker.z);
  destination.setOX(value.getOX());
  destination.setOY(value.getOY());
  destination.setOZ(value.getOZ());
  destination.setTheta(value.getTheta());
  motionServiceReq.setDestination(destination);

  // set SLAM resource name
  const slamResourceName = new commonApi.ResourceName();
  slamResourceName.setNamespace('rdk');
  slamResourceName.setType('service');
  slamResourceName.setSubtype('slam');
  slamResourceName.setName(props.name);
  motionServiceReq.setSlamServiceName(slamResourceName);

  // set component name
  const baseResourceName = new commonApi.ResourceName();
  baseResourceName.setNamespace('rdk');
  baseResourceName.setType('component');
  baseResourceName.setSubtype('base');
  baseResourceName.setName(baseResources[0]!.name);
  motionServiceReq.setComponentName(baseResourceName);

  // set extra as position-only constraint
  motionServiceReq.setExtra(
    Struct.fromJavaScript({
      motion_profile: 'position_only',
    })
  );

  props.client.motionService.moveOnMap(
    motionServiceReq,
    new grpc.Metadata(),
    (error: ServiceError | null, response: motionApi.MoveOnMapResponse | null) => {
      if (error) {
        moveClick = !moveClick;
        toast.error(`Error moving: ${error}`);
        return;
      }
      moveClick = !moveClick;
      toast.success(`MoveOnMap success: ${response!.getSuccess()}`);
    }
  );

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

  // we round to the tenths per figma design
  basePose.setX(Number(pose!.getX()!.toFixed(1)!));
  basePose.setY(Number(pose!.getY()!.toFixed(1)!));
  basePose.setZ(Number(pose!.getZ()!.toFixed(1)!));
  basePose.setOX(Number(pose!.getOX()!.toFixed(1)!));
  basePose.setOY(Number(pose!.getOY()!.toFixed(1)!));
  basePose.setOZ(Number(pose!.getOZ()!.toFixed(1)!));
  basePose.setTheta(Number(pose!.getTheta()!.toFixed(1)!));
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
      selected2dValue = 'manual';
      refreshErrorMessage2d = error !== null && typeof error === 'object' && 'message' in error
        ? `${refreshErrorMessage} ${error.message}`
        : `${refreshErrorMessage} ${error}`;
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
      selected3dValue = 'manual';
      refreshErrorMessage3d = error !== null && typeof error === 'object' && 'message' in error
        ? `${refreshErrorMessage} ${error.message}`
        : `${refreshErrorMessage} ${error}`;
      return;
    }
    if (refresh3DCancelled) {
      return;
    }
    scheduleRefresh3d(name, time);
  };
  slam3dTimeoutId = window.setTimeout(timeoutCallback, Number.parseFloat(time) * 1000);
};

const updateSLAM2dRefreshFrequency = async (name: string, time: 'manual' | string) => {
  refresh2DCancelled = true;
  window.clearTimeout(slam2dTimeoutId);
  refreshErrorMessage2d = null;
  refreshErrorMessage3d = null;

  if (time === 'manual') {
    try {
      const res = await refresh2d(name);
      handleRefresh2dResponse(res);
    } catch (error) {
      handleError('refresh2d', error);
      selected2dValue = 'manual';
      refreshErrorMessage2d = error !== null && typeof error === 'object' && 'message' in error
        ? `${refreshErrorMessage} ${error.message}`
        : `${refreshErrorMessage} ${error}`;
    }
  } else {
    refresh2DCancelled = false;
    scheduleRefresh2d(name, time);
  }
};

const updateSLAM3dRefreshFrequency = async (name: string, time: 'manual' | string) => {
  refresh3DCancelled = true;
  window.clearTimeout(slam3dTimeoutId);
  refreshErrorMessage2d = null;
  refreshErrorMessage3d = null;

  if (time === 'manual') {
    try {
      const res = await fetchSLAMMap(name);
      handleRefresh3dResponse(res);
    } catch (error) {
      handleError('fetchSLAMMap', error);
      selected3dValue = 'manual';
      refreshErrorMessage3d = error !== null && typeof error === 'object' && 'message' in error
        ? `${refreshErrorMessage} ${error.message}`
        : `${refreshErrorMessage} ${error}`;
    }
  } else {
    refresh3DCancelled = false;
    scheduleRefresh3d(name, time);
  }
};

const toggle3dExpand = () => {
  show3d = !show3d;
  if (!show3d) {
    selected3dValue = 'manual';
    return;
  }
  updateSLAM3dRefreshFrequency(props.name, selected3dValue);
};

const toggle2dExpand = () => {
  show2d = !show2d;
  updateSLAM2dRefreshFrequency(props.name, show2d ? selected2dValue : 'off');
};

const selectSLAM2dRefreshFrequency = () => {
  updateSLAM2dRefreshFrequency(props.name, selected2dValue);
};

const selectSLAMPCDRefreshFrequency = () => {
  updateSLAM3dRefreshFrequency(props.name, selected3dValue);
};

const refresh2dMap = () => {
  updateSLAM2dRefreshFrequency(props.name, 'manual');
};

const refresh3dMap = () => {
  updateSLAM3dRefreshFrequency(props.name, 'manual');
};

const handle2dRenderClick = (event: THREE.Vector3) => {
  updatedDest = true;
  destinationMarker = event;
};

const handleUpdateDestX = (event: CustomEvent<{ value: string }>) => {
  destinationMarker.x = Number.parseFloat(event.detail.value);
  updatedDest = true;
};

const handleUpdateDestY = (event: CustomEvent<{ value: string }>) => {
  destinationMarker.y = Number.parseFloat(event.detail.value);
  updatedDest = true;
};

const baseCopyPosition = () => {
  copyToClipboardWithToast(JSON.stringify(basePose.toObject()));
};

const executeDeleteDestinationMarker = () => {
  updatedDest = false;
  destinationMarker = new THREE.Vector3();
};

const toggleAxes = () => {
  showAxes = !showAxes;
};

onMounted(() => {
  props.statusStream?.on('end', () => {
    window.clearTimeout(slam2dTimeoutId);
    window.clearTimeout(slam3dTimeoutId);
  });
});

onUnmounted(() => {
  window.clearTimeout(slam2dTimeoutId);
  window.clearTimeout(slam3dTimeoutId);
});

</script>

<template>
  <v-collapse
    :title="props.name"
    class="slam"
    @toggle="toggle2dExpand()"
  >
    <v-breadcrumbs
      slot="title"
      crumbs="slam"
    />
    <div class="flex flex-wrap gap-4 border border-t-0 border-border-1 sm:flex-nowrap">
      <div class="flex min-w-fit flex-col gap-4 p-4">
        <div class="float-left pb-4">
          <div class="flex">
            <div class="w-64">
              <p class="mb-1 font-bold text-gray-800">
                Map
              </p>
              <div class="relative">
                <p class="mb-1 text-xs text-gray-500 ">
                  Refresh frequency
                </p>
                <select
                  v-model="selected2dValue"
                  class="
                      m-0 w-full appearance-none border border-solid border-black bg-white bg-clip-padding px-3 py-1.5
                      text-xs font-normal text-gray-700 focus:outline-none
                    "
                  aria-label="Default select example"
                  @change="selectSLAM2dRefreshFrequency()"
                >
                  <option
                    value="manual"
                    class="pb-5"
                  >
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
            <div class="px-2 pt-11">
              <v-button
                label="Refresh"
                icon="refresh"
                @click="refresh2dMap()"
              />
            </div>
          </div>
          <hr class="my-4 border-t border-gray-400">
          <div class="flex flex-row">
            <p class="mb-1 pr-52 font-bold text-gray-800">
              Ending Position
            </p>
            <v-icon
              name="trash"
              @click="executeDeleteDestinationMarker()"
            />
          </div>
          <div class="flex flex-row pb-2">
            <v-input
              type="number"
              label="x"
              incrementor="slider"
              :value="destinationMarker.x"
              step="0.1"
              @input="handleUpdateDestX($event)"
            />
            <v-input
              class="pl-2"
              type="number"
              label="y"
              incrementor="slider"
              :value="destinationMarker.z"
              step="0.1"
              @input="handleUpdateDestY($event)"
            />
          </div>
          <v-button
            class="pt-1"
            label="Move"
            variant="success"
            icon="play-circle-filled"
            :disabled="moveClick ? 'false' : 'true'"
            @click="executeMoveOnMap()"
          />
          <v-switch
            class="pt-2"
            label="Show Axes"
            :value="showAxes ? 'on' : 'off'"
            @input="toggleAxes()"
          />
        </div>
      </div>
      <div class="justify-start gap-4x border-border-1 sm:border-l">
        <div
          v-if="refreshErrorMessage2d && show2d"
          class="border-l-4 border-red-500 bg-gray-100 px-4 py-3"
        >
          {{ refreshErrorMessage2d }}
        </div>
        <div v-if="loaded2d && show2d">
          <div class="flex flex-row pl-5 pt-3">
            <div class="flex flex-col">
              <p class="text-xs">
                Current Position
              </p>
              <div class="flex flex-row items-center">
                <p class="items-end pr-2 text-xs text-gray-500">
                  x
                </p>
                <p>{{ basePose.getX() }}</p>

                <p class="pl-9 pr-2 text-xs text-gray-500">
                  y
                </p>
                <p>{{ basePose.getY() }}</p>

                <p class="pl-9 pr-2 text-xs text-gray-500">
                  z
                </p>
                <p>{{ basePose.getZ() }}</p>
              </div>
            </div>
            <div class="flex flex-col pl-10">
              <p class="text-xs">
                Current Orientation
              </p>
              <div class="flex flex-row items-center">
                <p class="pr-2 text-xs text-gray-500">
                  o<sub>x</sub>
                </p>
                <p>{{ basePose.getOX() }}</p>

                <p class="pl-9 pr-2 text-xs text-gray-500">
                  o<sub>y</sub>
                </p>
                <p>{{ basePose.getOY() }}</p>

                <p class="pl-9 pr-2 text-xs text-gray-500">
                  o<sub>z</sub>
                </p>
                <p>{{ basePose.getOZ() }}</p>

                <p class="pl-9 pr-2 text-xs text-gray-500">
                  &theta;
                </p>
                <p>{{ basePose.getTheta() }}</p>
              </div>
            </div>
            <div class="pl-4 pt-2">
              <v-icon
                name="copy"
                @click="baseCopyPosition()"
              />
            </div>
          </div>
          <Slam2dRender
            :point-cloud-update-count="pointCloudUpdateCount"
            :pointcloud="pointcloud"
            :pose="pose"
            :name="name"
            :resources="resources"
            :client="client"
            :dest-exists="updatedDest"
            :dest-vector="destinationMarker"
            :axes-visible="showAxes"
            @click="handle2dRenderClick($event)"
          />
        </div>
      </div>
    </div>
    <div class="pt-4">
      <div class="flex items-center gap-2">
        <v-switch
          :value="show3d ? 'on' : 'off'"
          @input="toggle3dExpand()"
        />
        <span class="pr-2">View SLAM Map (3D)</span>
      </div>
      <div
        v-if="refreshErrorMessage3d && show3d"
        class="border-l-4 border-red-500 bg-gray-100 px-4 py-3"
      >
        {{ refreshErrorMessage3d }}
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
                      border-border-1 m-0 w-full appearance-none border border-solid bg-white
                      bg-clip-padding px-3 py-1.5 text-xs font-normal text-gray-700 focus:outline-none"
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
  </v-collapse>
</template>
