<template functional>
  <component
    :is="props.tag"
    :ref="data.ref"
    class="bg-gray-100 dark:bg-gray-800 space-x-1"
    :class="[data.class, data.staticClass]"
    :style="[data.style, data.staticStyle]"
    v-bind="data.attrs"
    v-on="listeners"
  >
    <slot />
  </component>
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";

@Component({
  components: {},
})
export default class ViamTabs extends Vue {
  @Prop({ default: "nav" }) tag?: string;
}
</script>
