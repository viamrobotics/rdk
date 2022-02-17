<template>
  <div class="container mx-auto bg-white border-grey-light border mt-20">
    <div class="camera" v-for="streamName in streamNames" v-bind:key="streamName">
      <accordion :title="streamName">
        <div
          class="embed-responsive embed-responsive-16by9 relative w-full overflow-hidden"
        >
          <div class="flex justify-center">
            <div class="form-check form-switch">
              <input class="form-check-input appearance-none w-9 -ml-10 rounded-full float-left h-5 align-top bg-white bg-no-repeat bg-contain bg-gray-300 focus:outline-none cursor-pointer shadow-sm" type="checkbox" role="switch" id="flexSwitchCheckDefault">
              <label class="form-check-label inline-block text-gray-800" for="flexSwitchCheckDefault">Video</label>
            </div>
          </div>
          <div :id="'stream-' + streamName">
            <video autoplay="" playsinline=""></video>
          </div>
        </div>
      </accordion>
    </div>
  </div>
</template>

<script lang="ts">
import {Component, Prop, Vue, Watch} from "vue-property-decorator";
import "vue-class-component/hooks";
import Accordion from "./Accordion.vue";

@Component({
  components: {
    Accordion,
  },
})
export default class Camera extends Vue {
  @Prop() streamNames!: [string];

  selected: string | undefined = "";

  mounted(): void {
    this.selected = this.defaultOption;
  }

  isDisabled(option: string): boolean {
    // return !!this.disabledOptions?.includes(option);
  }

  viewCamera(option: string): void {
    this.selected = option;
    this.$emit("selectOption", option);
  }
}
</script>

<style scoped>

</style>