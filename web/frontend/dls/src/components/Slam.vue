<template>
  <div>
    <Collapse>
      <div class="flex">
        <h2 class="p-4 text-xl">SLAM</h2>
      </div>
      <div class="flex mb-4">
        <table
          border="1"
          style="margin-right: 1em"
          class="border border-slate-300"
        >
          <tr>
            <th colspan="1" class="border border-slate-300">
              <h5>
                View SLAM Map (JPEG)
                <ViamButton
                  color="black"
                  group
                  variant="primary"
                  @click="refreshImageMap()"
                >
                  Refresh
                </ViamButton>
              </h5>
            </th>
          </tr>
          <tr>
            <th colspan="1" class="border border-slate-300">
              <img
                v-if="imageMap !== ''"
                :src="imageMap"
                width="500"
                height="500"
              />
            </th>
          </tr>
        </table>
        <table
          border="1"
          style="margin-right: 1em"
          class="border border-slate-300"
        >
          <tr>
            <th colspan="1" class="border border-slate-300">
              <h5>
                View SLAM Map (PCD)
                <ViamButton
                  color="black"
                  group
                  variant="primary"
                  @click="refreshPCDMap()"
                >
                  Refresh
                </ViamButton>
              </h5>
            </th>
          </tr>
          <tr>
            <th colspan="1" class="border border-slate-300">
              <div id="pcd" width="400" height="400" />
            </th>
          </tr>
        </table>
      </div>
    </Collapse>
  </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";
import "vue-class-component/hooks";
import Collapse from "./Collapse.vue";
import ViamButton from "./Button.vue";

@Component({
  components: {
    Collapse,
    ViamButton,
  },
})
export default class Slam extends Vue {
  @Prop({ default: "" }) imageMap = "";
  @Prop({ default: null }) pcdMap = null;

  refreshImageMap(): void {
    this.$emit("refresh-image-map");
  }
  refreshPCDMap(): void {
    this.$emit("refresh-pcd-map");
  }
}
</script>

<style scoped></style>
