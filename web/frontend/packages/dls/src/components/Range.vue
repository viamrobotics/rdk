<template>
    <div class="relative pt-1">
    <label for="customRange1" class="form-label">{{name}}</label>
        <input
            type="range"
            class="
            appearance-none w-full h-0.5 bg-black rounded outline-none
            "
            id="customRange1"
            :min="min" :max="max" v-model="value" :step="step"
        />
        <input 
            type="number" 
            class="appearance-none block w-full border text-gray-700 dark:text-gray-200 placeholder-gray-400 leading-tight dark:placeholder-gray-500
            focus:outline-none transition-colors duration-150 ease-in-out"
            v-model="value" 
        />
    </div>  
</template>
<script>
import Vue from 'vue';
export default Vue.extend({
  props: {
    max: {
      type: Number,
      default: 100,
    },
    min: {
      type: Number,
      default: 0,
    },
    step: {
      type: Number,
      default: 10,
    },
    name: {
      type: String,
      default: null,
    },
    percentage: {
      type: Number,
      default: 0,
      validator: (val) => val >= 0 && val <= 100,
    },
  },
  data() {
    return {
      value: this.percentage
    };
  },
});
</script>
<style scoped>
</style>