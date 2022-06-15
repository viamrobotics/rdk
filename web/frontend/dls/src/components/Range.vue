<template>
  <div class="flex">
    <div class="flex-auto grid">
      <label class="text-xs">{{ name }}</label>
      <div class="flex items-center pt-1">
        <div class="text-xs px-2 text-center">{{ min }}{{ unit }}</div>
        <div class="pt-1 w-64">
          <vue-slide-bar
            v-model="innerValue"
            :line-height="2"
            :icon-width="16"
            paddingless
            :data="possibleValues"
            :labelStyles="{ color: '#9d9d9d', backgroundColor: '#9d9d9d' }"
            :processStyle="processStyle"
          >
            <div slot="tooltip" class="w-4 h-4">
              <div
                class="border border-black rounded-full w-4 h-4 mt-5 bg-white range-tooltip"
              ></div>
            </div>
          </vue-slide-bar>
        </div>
        <div class="px-2 text-xs text-center">{{ max }}{{ unit }}</div>
        <number-input
          class="w-12"
          v-model="innerValue"
          :small="true"
          :id="id"
          :readonly="false"
        ></number-input>
      </div>
    </div>
  </div>
</template>
<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';
import NumberInput from './NumberInput.vue';
import VueSlideBar from 'vue-slide-bar';

@Component({
  components: {
    NumberInput,
    VueSlideBar,
  },
})
export default class ViamRange extends Vue {
  @Prop({ default: 100, type: Number }) max!: number;
  @Prop({ default: 0, type: Number }) min!: number;
  @Prop({ default: 10, type: Number }) step!: number;
  @Prop({ default: '' }) name!: string;
  @Prop({ default: 'DefaultId' }) id!: string;
  @Prop({ default: '' }) unit!: string;
  @Prop({ required: true, type: Number }) value!: number;
  @Prop({ default: false, type: Boolean }) hideTickLabels!: boolean;

  get innerValue(): number {
    return this.value;
  }
  set innerValue(value: number) {
    this.$emit('input', value);
  }

  get rangeLabels(): { label: number; isHide: boolean }[] | null {
    if (!this.possibleValues) return null;
    return this.possibleValues.map((el) => ({
      label: el,
      isHide: true,
    }));
  }
  get possibleValues(): number[] | null {
    if (this.hideTickLabels) return null;

    let count = Math.floor((this.max - this.min) / this.step) + 1;
    let result = [];
    for (let i = 0; i < count; i++) {
      result.push(this.min + i * this.step);
    }
    return result;
  }
  get processStyle(): { [key: string]: string } {
    return {
      backgroundColor: '#000000',
      height: '4px',
      'border-radius': '0',
      top: '-2px',
    };
  }
}
</script>
<style>
.vue-slide-bar-range {
  position: absolute;
  width: 100%;
  top: -10px;
}
.vue-slide-bar {
  background-color: #9d9d9d !important;
  border-radius: 0 !important;
}
.vue-slide-bar-separate {
  width: 1px !important;
  height: 4px !important;
}
</style>
