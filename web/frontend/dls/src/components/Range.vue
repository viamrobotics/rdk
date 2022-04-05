<template>
  <div class="flex">
    <div class="flex-auto grid">
      <label for="customRange1" class="text-xs">{{ name }}</label>
      <div class="flex">
        <div class="pr-3">
          {{ min }}
        </div>
        <div class="pt-1 w-64">
          <input
            type="range"
            class="appearance-none w-full h-0.5 bg-black rounded outline-none"
            id="customRange1"
            :min="min"
            :max="max"
            v-model="value"
            :step="step"
          />
        </div>
        <div class="pl-3 pr-2">{{ max }}mm</div>
        <ViamInput
          class="w-12"
          color="primary"
          group="False"
          variant="primary"
          v-model="value"
        ></ViamInput>
      </div>
    </div>
  </div>
</template>
<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";
import ViamInput from "./ViamInput.vue";

@Component({
  components: {
    ViamInput,
  },
})
export default class ViamRange extends Vue {
  @Prop({ default: 100 }) max?: number;
  @Prop({ default: 0 }) min?: number;
  @Prop({ default: 10 }) step?: number;
  @Prop({ default: null }) name?: string;
  @Prop({ default: 0 }) percentage?: number | undefined = 0;

  value: number | undefined = this.percentage;
}
</script>
<style scoped></style>
