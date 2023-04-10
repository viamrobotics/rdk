
<script setup lang="ts">

import { $ref, $computed } from 'vue/macros';
import { grpc } from '@improbable-eng/grpc-web';
import { toast } from '../lib/toast';
import * as THREE from 'three';
import { Client, commonApi, ResponseStream, ServiceError, slamApi, motionApi } from '@viamrobotics/sdk';
import { displayError, isServiceError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';
import { copyToClipboardWithToast } from '../lib/copy-to-clipboard';
import Slam2dRender from './slam-2d-render.vue';
import { filterResources } from '../lib/resource';

type MapAndPose = { map: Uint8Array, pose: commonApi.Pose}

const props = defineProps<{
  name: string
  resources: commonApi.ResourceName.AsObject[]
  client: Client
}>();

const selected2dValue = $ref('manual');
let pointCloudUpdateCount = $ref(0);
let pointcloud = $ref<Uint8Array | undefined>();
let pose = $ref<commonApi.Pose | undefined>();
let show2d = $ref(false);
let showAxes = $ref(true);
let refresh2DCancelled = true;
let updatedDest = $ref(false);
let threeJPos = $ref(new THREE.Vector3( ));
let moveClick = $ref(true);
let x = 0;
let y = 0;
let z = 0;
let o_x = 0;
let o_y = 0;
let o_z = 0;
let theta = 0;

const loaded2d = $computed(() => (pointcloud !== undefined && pose !== undefined));

let slam2dTimeoutId = -1;

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

const executeMove = async () => {
  moveClick = false
  const req = new motionApi.MoveRequest();
  // set request name
  req.setName("builtin");

  const value = await fetchSLAMPose(props.name)  
  // set pose in frame
  const sentPose = new commonApi.Pose
  sentPose.setX(Math.abs(threeJPos.x - value.getX()))
  sentPose.setY(Math.abs(threeJPos.z - value.getY()))
  sentPose.setZ(value.getZ())
  sentPose.setOX(value.getOX())
  sentPose.setOY(value.getOY())
  sentPose.setOZ(value.getOZ())
  sentPose.setTheta(value.getTheta())

  let pif = new commonApi.PoseInFrame
  pif.setReferenceFrame("world");
  pif.setPose(sentPose);
  req.setDestination(pif);

  //set SLAM resource name
  const slamResourceName = new commonApi.ResourceName
  slamResourceName.setNamespace("rdk")
  slamResourceName.setType("service")
  slamResourceName.setSubtype("slam")
  slamResourceName.setName(props.name)
  req.setSlamServiceName(slamResourceName)

  //set component name
  const baseResource = filterResources(props.resources, 'rdk', 'component', 'base')
  const baseResourceName = new commonApi.ResourceName
  baseResourceName.setNamespace("rdk")
  baseResourceName.setType("component")
  baseResourceName.setSubtype("base")
  baseResourceName.setName(baseResource[0]!.name)
  req.setComponentName(baseResourceName)
  
  // set worldstate
  const wrldst = new commonApi.WorldState
  req.setWorldState(wrldst)

  // set constraints
  req.setConstraints(undefined)

  // execute the actual call to validate e2e plumbing
  props.client.motionService.move(req, new grpc.Metadata(), (error: ServiceError, response: motionApi.MoveRequestResponse) => {
    if (error) {
      toast.error(`Error moving: ${error}`);
      return;
    }
    toast.success(`Move success: ${response!.getSuccess()}`);
    return;
  });
  
}

const executeStopMove =  () => {
  console.log('executeStopMove')
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
  x = +pose?.getX()!.toFixed(1)!;
  y = +pose?.getY()!.toFixed(1)!;
  z = +pose?.getZ()!.toFixed(1)!;
  o_x = +pose?.getOX()!.toFixed(1)!;
  o_y = +pose?.getOY()!.toFixed(1)!;
  o_z = +pose?.getOZ()!.toFixed(1)!;
  theta = +pose?.getTheta()!.toFixed(1)!;
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

const toggle2dExpand = () => {
  show2d = !show2d;
  updateSLAM2dRefreshFrequency(props.name, show2d ? selected2dValue : 'off');
};

const selectSLAM2dRefreshFrequency = () => {
  updateSLAM2dRefreshFrequency(props.name, selected2dValue);
};

const refresh2dMap = () => {
  updateSLAM2dRefreshFrequency(props.name, selected2dValue);
};

const handle2dRenderClick = (event: THREE.Vector3) => {
  updatedDest = true
  threeJPos = event
}

const handleUpdateX = (event: CustomEvent<{ value: string }>) => {
  threeJPos.x = Number.parseFloat(event.detail.value)
  updatedDest = true
}

const handleUpdateY = (event: CustomEvent<{ value: string }>) => {
  threeJPos.z = Number.parseFloat(event.detail.value)
  updatedDest = true
}

const baseCopyPosition = () => {
  const position: JSON = <JSON><unknown>{
    "x": x,
    "y": y,
    "z": z,
    "o_x": o_x,
    "o_y": o_y,
    "o_z": o_z,
    "theta": theta
  }
  const asString = JSON.stringify(position)
  copyToClipboardWithToast(asString)
};

const executeDelete = () => {
  updatedDest = false
  threeJPos = new THREE.Vector3()
}

const toggleAxes = () => {
  showAxes = !showAxes
}

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
    <v-button
        slot="header"
        icon="stop-circle"
        variant="danger"
        label="STOP"
        :disabled="moveClick ? 'true' : 'false'"
        @click="executeStopMove()"
    />
    <div class="flex flex-wrap sm:flex-nowrap gap-4 border border-t-0 border-black">
        <div class="flex flex-col gap-4 p-4 min-w-fit">
          <div class="float-left pb-4">
            <div class="flex">
              <div class="w-64">
                <p class="mb-1 text-gray-800 font-bold">
                  Map
                </p>
                <div class="relative">
                  <p class="text-xs mb-1 text-gray-500 ">
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
                    <option value="manual" class="pb-5">
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
              <p class="mb-1 text-gray-800 font-bold pr-52">
                  Ending Position
              </p>
              <v-icon
                name='trash'
                @click="executeDelete()"
              />
            </div>
            <div class="flex flex-row pb-2">
              
              <v-input
                type='number'
                label='x'
                incrementor='slider'
                :value="threeJPos.x"
                step='0.1'
                @input="handleUpdateX($event)"
              />
              <v-input
                class="pl-2"
                type='number'
                label='y'
                incrementor='slider'
                :value="threeJPos.z"
                step='0.1'
                @input="handleUpdateY($event)"
              />

            </div>
            <v-button  
              class="pt-1"
              label="Move"
              variant="success"
              icon="play-circle-filled"
              @click="executeMove()"
            />
            <v-switch
            class="pt-2"
            label="Show Axes"
            :value="showAxes ? 'on' : 'off'"
            @input="toggleAxes()"
            />
          </div>
        </div>
        <div class="justify-start gap-4 sm:border-l border-black">
        <div v-if="loaded2d && show2d">
          <div class="flex flex-row pl-5 pt-3">
            <div class="flex flex-col">
              <p class="text-xs"> Current Position </p>
                <div class="flex flex-row">
                  <p class="pr-2 text-xs items-end"> x </p>
                  <p >{{ x }}</p>
                  <p class="pl-7 pr-2 text-xs"> y </p>
                  <p>{{ y }}</p>
                  <p class="pl-7 pr-2 text-xs"> z </p>
                  <p>{{ z }}</p>
                </div>
            </div>
            <div class="flex flex-col pl-10">
              <p class="text-xs"> Current Orientation </p>
              <div class="flex flex-row">
                <p class="pr-2 text-xs"> o_x </p>
                <p>{{ o_x }}</p>
                <p class="pl-7 pr-2 text-xs"> o_y </p>
                <p>{{ o_y }}</p>
                <p class="pl-7 pr-2 text-xs"> o_z </p>
                <p>{{ o_z }}</p>
                <p class="pl-7 pr-2 text-xs"> &theta; </p>
                <p>{{ theta }}</p>
              </div>
            </div>
            <div class="pl-4 pt-2">
              <v-icon
                name="copy"
                @click="baseCopyPosition()"
              >
              </v-icon>
            </div>
          </div>
          <Slam2dRender
              :point-cloud-update-count="pointCloudUpdateCount"
              :pointcloud="pointcloud"
              :pose="pose"
              :name="name"
              :resources="resources"
              :client="client"
              :dest-exists='updatedDest'
              :dest-vector='threeJPos'
              :axes="showAxes"
              @click="handle2dRenderClick($event)"
           />
        </div>
      </div>
    </div>
  </v-collapse>
</template>
