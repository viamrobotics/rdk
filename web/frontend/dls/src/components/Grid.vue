<template functional>
  <component
    :is="props.tag"
    :ref="data.ref"
    class="grid"
    :class="[
      {
        [`grid-cols-1 sm:grid-cols-${props.cols}`]: props.cols,
        [`gap-1 sm:gap-${props.gap}`]: props.gap,
      },
      data.class,
      data.staticClass,
    ]"
    :style="[data.style, data.staticStyle]"
    v-bind="data.attrs"
    v-on="listeners"
  >
    <slot />
  </component>
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";
import "vue-class-component/hooks";

@Component
export default class Grid extends Vue {
  @Prop({ default: null }) cols?: string;
  @Prop({ default: null }) gap?: string;
  @Prop({ default: "div" }) tag?: string;
}
</script>
