<template functional>
  <component
    :is="props.tag"
    :ref="data.ref"
    class="tabs-root-container relative bg-gray-100 dark:bg-gray-800 px-5 border-grey border-t border-b h-8"
    :class="[data.class, data.staticClass]"
    :style="[data.style, data.staticStyle]"
    v-bind="data.attrs"
    v-on="listeners"
  >
    <div class="tabs-container absolute flex">
      <slot />
    </div>
  </component>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

@Component({
  components: {},
})
export default class ViamTabs extends Vue {
  @Prop({ default: 'nav' }) tag?: string;
}
</script>
<style>
.tabs-container {
  top: -1px;
}
.tabs-root-container {
  height: 34px;
  border-bottom-color: black;
}
</style>
