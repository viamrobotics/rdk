<template>
  <div>
    <Collapse>
      <div class="flex">
        <h2 class="p-4 text-xl">{{ streamName }}</h2>
        <Breadcrumbs :crumbs="crumbs" disabled="true"></Breadcrumbs>
      </div>
      <template v-slot:content>
        <div
          class="border-l border-r border-b border-black p-2"
          :style="{ height: height }"
        >
          <Container>
            <div class="pt-4">
              <span class="pr-2">View SLAM Map</span>
              <div class="float-right pb-4">
                <div class="flex">
                  <div class="w-64">
                  </div>
                  <div class="pl-2 pr-2 pt-7">
                    <ViamButton
                      color="black"
                      group
                      variant="primary"
                      @click="refreshMap()"
                    >
                      <template v-slot:icon>
                        <ViamIcon color="white" :path="mdiRestore"
                          >Refresh</ViamIcon
                        >
                      </template>
                      Refresh
                    </ViamButton>
                  </div>
                </div>
              </div>
              <div
                class="clear-both h-fit transition-all duration-300 ease-in-out"
                :id="streamId"
              ></div>
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
import ViamInfoButton from "./ViamInfoButton.vue";

import RadioButtons from "./RadioButtons.vue";
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
    ViamIcon,
    RadioButtons,
    ViamInfoButton,
  },
})
export default class Base extends Vue {
  @Prop({ default: null }) streamName!: string;
  @Prop({ default: null }) crumbs!: [string];
  @Prop({ default: true }) connectedCamera!: boolean;
  @Prop({ default: true }) connectedPCD!: boolean;
  @Prop({ default: 0 }) x = 0;
  @Prop({ default: 0 }) y = 0;
  @Prop({ default: 0 }) z = 0;

  mdiInformationOutline = mdiInformationOutline;
  mdiDownloadOutline = mdiDownloadOutline;
  mdiCameraIris = mdiCameraIris;
  mdiImageFilterCenterFocus = mdiImageFilterCenterFocus;
  mdiRestore = mdiRestore;

  camera = !this.connectedCamera;
  pcd = !this.connectedPCD;
  height = "auto";
  selectedValue = "live";
  streamId = "stream-" + this.streamName;
  //pcdId = "pcd-" + this.streamName;
  selected = "";
  speed = 0;
  min = 0;
  max = 500;


  refreshMap(): void {
    this.$emit("refresh-map", this.selectedValue);
  }
}
</script>

<style scoped></style>
