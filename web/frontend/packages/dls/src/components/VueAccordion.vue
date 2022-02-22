<template>
  <transition
    @enter="onEnter"
    @after-enter="onAfterEnter"
    @before-leave="onBeforeLeave"
    @leave="onLeave"
  >
    <div
      class="vue-accordion"
      ref="wrapper"
      :style="wrapperStyle"
      v-if="expanded"
    >
      <div class="vue-accordion__inner" ref="inner">
        <slot></slot>
      </div>
    </div>
  </transition>
</template>

<script>
import Vue from 'vue'
export default Vue.extend({
  props: {
    expanded: {
      type: Boolean,
      required: true,
    },
    duration: {
      type: Number,
      default: 250,
    },
  },
  computed: {
    wrapperStyle() {
      return {
        transitionDuration: `${this.duration}ms`,
      }
    },
  },
  mounted() {
    if (this.expanded) {
      this.setWrapperHeightTo('auto')
    }
  },
  methods: {
    onEnter(el) {
      this.setWrapperHeightTo(this.getContentHeight(), el)
    },
    onAfterEnter(el) {
      this.setWrapperHeightTo('auto', el)
    },
    onBeforeLeave(el) {
      this.setWrapperHeightTo(this.getContentHeight(), el)
    },
    onLeave(el) {
      el.scrollHeight
      this.setWrapperHeightTo(0, el)
    },
    getContentHeight() {
      const inner = this.$refs.inner
      return inner.getBoundingClientRect().height
    },
    /**
     * @param {number | 'auto'} height
     */
    setWrapperHeightTo(height, el = this.$refs.wrapper) {
      el.style.height = typeof height === 'number' ? `${height}px` : height
    },
  },
})
</script>

<style >
.vue-accordion {
  transition-timing-function: ease;
  transition-property: height;
  height: 0;
  overflow: hidden;
}
.vue-accordion__inner {
  display: table;
  width: 100%;
}
</style>