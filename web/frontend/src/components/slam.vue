
<script setup lang="ts">

// import { nextTick } from 'vue';
import { grpc } from '@improbable-eng/grpc-web';
import { Client, commonApi, slamApi } from '@viamrobotics/sdk';
import { displayError } from '../lib/error';
import { rcLogConditionally } from '../lib/log';

/*
 * import PCD from './pcd/pcd-view.vue';
 * import Slam2dRender from './slam-2d-render.vue';
 */

const props = defineProps<{
  name: string
  resources: commonApi.ResourceName.AsObject[]
  client: Client
}>();

let imageMap = $ref('');
const selected2dValue = $ref('manual');
let show2d = $ref(false);
let slam2dIntervalId = -1;

/*
 * const selected3dValue = $ref('manual');
 * let pointCloudUpdateCount = $ref(0);
 * let pointcloud = $ref<Uint8Array | undefined>();
 * let pose = $ref<commonApi.Pose | undefined>();
 * let show3d = $ref(false);
 * const loaded2d = $computed(() => (pointcloud !== undefined && pose !== undefined));
 * let slam3dIntervalId = -1;
 */

/*
 * const concatArrayU8 = (arrays: Uint8Array[]) => {
 *   const totalLength = arrays.reduce((acc, value) => acc + value.length, 0);
 *   const result = new Uint8Array(totalLength);
 *   let length = 0;
 *   for (const array of arrays) {
 *     result.set(array, length);
 *     length += array.length;
 *   }
 *   return result;
 * };
 */

const fetchSLAMImageMap = (name: string) => {
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

/*
 * const fetchSLAMMap = (name: string) => {
 *   return nextTick(() => {
 *     const req = new slamApi.GetPointCloudMapStreamRequest();
 *     req.setName(name);
 *     rcLogConditionally(req);
 *     const chunks: Uint8Array[] = [];
 *     const getPointCloudMapStream = props.client.slamService.getPointCloudMapStream(req);
 *     getPointCloudMapStream.on('data', (res) => {
 *       const chunk = res.getPointCloudPcdChunk_asU8();
 *       chunks.push(chunk);
 *     });
 *     getPointCloudMapStream.on('status', (status) => {
 *       if (status.code !== 0) {
 *         const error = {
 *           message: status.details,
 *           code: status.code,
 *           metadata: status.metadata,
 *         };
 *         displayError(error);
 *       }
 *     });
 *     getPointCloudMapStream.on('end', (end) => {
 *       if (end === undefined || end.code !== 0) {
 *         // the error will be logged in the 'status' callback
 *         return;
 *       }
 *       pointcloud = concatArrayU8(chunks);
 *       pointCloudUpdateCount += 1;
 *     });
 *   });
 * };
 */

/*
 * const fetchSLAMPose = (name: string) => {
 *   return nextTick(() => {
 *     const req = new slamApi.GetPositionNewRequest();
 *     req.setName(name);
 *     props.client.slamService.getPositionNew(req, new grpc.Metadata(), (error, res): void => {
 *       if (error) {
 *         displayError(error);
 *         return;
 *       }
 *       pose = res!.getPose()!;
 *     });
 *   });
 * };
 */

const refresh2d = async (name: string) => {

  /*
   * const mapPromise = fetchSLAMMap(name);
   * const posePromise = fetchSLAMPose(name);
   * await mapPromise;
   * await posePromise;
   */
  await fetchSLAMImageMap(name);
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

/*
 * const updateSLAM3dRefreshFrequency = (name: string, time: 'manual' | 'off' | string) => {
 *   clearInterval(slam3dIntervalId);
 *   if (time === 'manual') {
 *     fetchSLAMMap(name);
 *   } else if (time === 'off') {
 *     // do nothing
 *   } else {
 *     fetchSLAMMap(name);
 *     slam3dIntervalId = window.setInterval(() => {
 *       fetchSLAMMap(name);
 *     }, Number.parseFloat(time) * 1000);
 *   }
 * };
 */

const toggle2dExpand = () => {
  show2d = !show2d;
  updateSLAM2dRefreshFrequency(props.name, show2d ? selected2dValue : 'off');
};

/*
 * const toggle3dExpand = () => {
 *   show3d = !show3d;
 *   updateSLAM3dRefreshFrequency(props.name, show3d ? selected3dValue : 'off');
 * };
 */

const selectSLAM2dRefreshFrequency = () => {
  updateSLAM2dRefreshFrequency(props.name, selected2dValue);
};

/*
 * const selectSLAMPCDRefreshFrequency = () => {
 *   updateSLAM3dRefreshFrequency(props.name, selected3dValue);
 * };
 */

// eslint-disable-next-line require-await
const refresh2dMap = async () => {
  updateSLAM2dRefreshFrequency(props.name, selected2dValue);
};

/*
 * const refresh3dMap = () => {
 *   updateSLAM3dRefreshFrequency(props.name, selected3dValue);
 * };
 */

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
          <img
            v-if="show2d"
            :src="imageMap"
            width="500"
            height="500"
          >
          <!-- <Slam2dRender
            v-if="loaded2d && show2d"
            :point-cloud-update-count="pointCloudUpdateCount"
            :pointcloud="pointcloud"
            :pose="pose"
            :name="name"
            :resources="resources"
            :client="client"
          /> -->
        </div>
        <!-- <div class="pt-4">
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
        </div> -->
      </div>
    </div>
  </v-collapse>
</template>
