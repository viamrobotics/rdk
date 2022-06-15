<template functional>
  <component
    :is="props.tag"
    :ref="data.ref"
    class="container mx-auto"
    :class="[data.class, data.staticClass]"
    :style="[data.style, data.staticStyle]"
    v-bind="data.attrs"
  >
    <slot />
  </component>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';
import 'vue-class-component/hooks';

@Component
export default class Container extends Vue {
  @Prop({ default: 'div' }) tag?: string;
}
</script>
