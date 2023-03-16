<script setup lang="ts">

import { computed } from 'vue';

const props = withDefaults(defineProps<{
  path: string
  title?: string
  size?: number | string
  rotate?: number | string
  color?: string
  spin?: boolean | string
}>(), {
  size: 18,
  rotate: 0,
  spin: false,
  color: '#000',
  title: '',
});

const rotation = computed(() => {
  return `rotate(${props.rotate} 12 12)`;
});

const isNumeric = (value: string) => {
  return (/^-{0,1}\d+$/u).test(value);
};

const iconSize = () => {
  return isNumeric(String(props.size)) ? `${props.size}px` : props.size;
};

</script>

<template>
  <svg
    :width="`${iconSize()}`"
    :height="`${iconSize()}`"
    viewBox="0 0 24 24"
    :fill="color"
  >
    <title v-if="title">{{ title }}</title>
    <g :transform="rotation">
      <path :d="path" />
    </g>
  </svg>
</template>
