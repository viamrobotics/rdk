<template>
  <div class="container mx-auto bg-white border-grey-light border mt-20">
    <div class="camera" v-for="streamName in streamNames" v-bind:key="streamName">
      <accordion :title="streamName">
        <div
          class="embed-responsive embed-responsive-16by9 relative w-full overflow-hidden"
        >
          <div :id="'stream-' + streamName">
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
    // this.selected = this.defaultOption;
  }

  isDisabled(option: string): boolean {
    return false;
  }

  viewCamera(option: string): void {
    this.selected = option;
    this.$emit("selectOption", option);
  }
}
</script>

<style scoped>

</style>