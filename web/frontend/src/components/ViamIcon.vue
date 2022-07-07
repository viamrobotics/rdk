<template>
  <svg
    :width="`${iconSize()}`"
    :height="`${iconSize()}`"
    viewBox="0 0 24 24"
    :fill="color"
  >
    <template v-if="title">
      <title>{{ title }}</title>
    </template>
    <template v-if="description">
      <description>{{ description }}</description>
    </template>
    <g :transform="rotation">
      <path :d="path" />
    </g>
  </svg>
</template>

<script>
export default {
  computed: {
    viewBox() {
      return `0 0 ${this.iconSize()} ${this.iconSize()}`;
    },
    rotation() {
      return `rotate(${this.rotate} 12 12)`;
    },
  },
  methods: {
    iconSize() {
      if (this.isNumeric(this.size)) {
        return `${this.size}px`;
      }
      return this.size;
    },
    isNumeric(value) {
      return /^-{0,1}\d+$/.test(value);
    },
  },
  props: {
    path: {
      type: String,
      required: true,
    },
    title: String,
    description: String,
    size: {
      type: [Number, String],
      default: 18,
    },
    horizontal: Boolean,
    vertical: Boolean,
    rotate: {
      type: [Number, String],
      default: 0,
    },
    color: {
      type: String,
    },
    spin: {
      type: Boolean || String,
      default: false,
    },
  },
};
</script>
