<template>
  <div class="home">
    <MotorDetail
      motorName="MOTOR NAME"
      :motorStatus="{
        on: false,
        positionSupported: true,
        position: 0,
        pidConfig: {
          fieldsMap: [
            ['Kd', { numberValue: 0 }],
            ['Kp', { numberValue: 0.34 }],
            ['Ki', { numberValue: 0.77 }],
          ],
        },
      }"
    />
    <div v-if="!loading">
    {{ cameraFrame }}
    </div>
  </div>
</template>

<script lang="ts">
import { Component, Vue } from "vue-property-decorator";
import MotorDetail from "@dls/components/MotorDetail.vue"; // @dls is an alias to dls/src

import CameraClientWrapper from "../services/cameraClientWrapper";

import {
    CameraServiceRenderFrameRequest,
    CameraServiceGetFrameResponse,
    CameraServiceGetObjectPointCloudsResponse
} from "proto/api/component/v1/camera_pb";

let client: CameraClientWrapper;
client = new CameraClientWrapper();

export default {
  components: {
    MotorDetail
  },
  data() {
    return {
      loading: true,
      cameraFrame: {
        mimeType: 'image/jpeg',
        frame: "",
        widthPx: 0,
        heightPx: 0,
      } as CameraServiceGetFrameResponse.AsObject
    }
  }
  // },
  // async mounted() {
  //   let cameraFrame = await this.renderFrame("test", "image/jpeg");
  //   if (cameraFrame === undefined) {
  //     return;
  //   }
  //   this.cameraFrame = this.cameraFrame.toObject();
  //   this.loading = false;
  // },
  // methods: {
  //   renderFrame: async function(name: string, mimetype: string) {
  //     try {
  //       let frame = await client.renderFrame(name, mimetype);
  //       this.$store.commit("updateCameraFrame", frame);
  //       return frame;
  //     } catch (err) {
  //       // this.$store.commit("updateCameraFrame", {
  //       //   error: "Could not load frame"
  //       // } as CameraServiceGetFrameResponse);
  //     }
  //   }
  // }
}
</script>
