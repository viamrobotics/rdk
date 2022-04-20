<template>
  <div class="flex">
    <div class="flex-auto grid">
      <label for="customRange1" class="text-xs">{{ name }}</label>
      <div class="flex items-center">
        <div class="pr-2">
          {{ min }}<span v-html="unit"></span>
        </div>
        <div class="pt-1 w-64">
           <vue-slide-bar v-model="innerValue"
                          :line-height="2"
                          :icon-width="16"
                          paddingless
                          :data="possibleValues"
                          :range="rangeLabels"
                          :labelStyles="{ color: '#9d9d9d', backgroundColor: '#9d9d9d' }"

                          :processStyle="{ backgroundColor: '#000000' }">
            <div slot="tooltip" class="w-4 h-4">
              <div class="border border-black rounded-full w-4 h-4 mt-5	bg-white range-tooltip"></div>
            </div>
          </vue-slide-bar>
        </div>
        <div class="pl-3 pr-2">{{ max }}<span v-html="unit"></span></div>
        <number-input
          class="w-9"
          :hideControls="true"
          v-model="innerValue"
        ></number-input>
      </div>
    </div>
  </div>
</template>
<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";
import NumberInput from "./NumberInput.vue";
import VueSlideBar from 'vue-slide-bar'

@Component({
  components: {
    NumberInput, VueSlideBar
  },
})
export default class ViamRange extends Vue {
  @Prop({ default: 100 }) max?: number;
  @Prop({ default: 0 }) min?: number;
  @Prop({ default: 10 }) step?: number;
  @Prop({ default: null }) name?: string;
  @Prop({ default: "DefaultId" }) id?: string;
  @Prop({ default: 0 }) percentage?: number | undefined = 0;
  @Prop({ default: "mm" }) unit?: string;
  @Prop({ required: true }) value!: number;
  @Prop({default: null}) possibleValues!: number[];
  @Prop({default: false}) hideTickLabels!: boolean;


  get innerValue(): number {
    return this.value;
  }
  set innerValue(value: number) {
    this.$emit("input", value);
  }

  get rangeLabels(): object[] {
    if (!this.possibleValues)
      return null
    return this.possibleValues.map(el=>({
      label: el,
      isHide: true,
    }))
  }
}
</script>
<style scoped>
input[type="range"] {
  height: 2px;
  -webkit-appearance: none;
  margin: 10px 0;
  width: 100%;
}
.range-tooltip{
  /* margin-left: 3px; */
}
</style>
<style>
  .vue-slide-bar-range {
    position: absolute;
    width: 100%;
    top: 30px;
  }
  .vue-slide-bar {
    background-color: #9D9D9D !important; 
    border-radius: none !important;
  }
  .vue-slide-bar-separate {
    width: 1px !important;
    height: 4px !important;
  }
</style>
