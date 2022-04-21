<template>
  <component
    :is="tag"
    :class="[
      'border-l duration-150 py-2 px-4 px-6 text-xs outline-none relative',
      {
        'border-t border-l font-bold border-r border-black bg-white dark:bg-gray-600 text-gray-900 dark:text-gray-50':
          selected,
        'border-r border-grey cursor-pointer hover:text-gray-700 dark:hover:text-gray-200':
          !selected && !disabled,
        'cursor-not-allowed text-gray-500': disabled && !selected,
      },
    ]"
    :aria-disabled="disabled ? 'true' : null"
    :aria-selected="selected ? 'true' : 'false'"
    :tabindex="selected || disabled ? null : 0"
    @click="$emit('select')"
    @keyup.enter="$emit('select')"
    @keyup.space="$emit('select')"
    v-on="$listeners"
  >
  <div><slot></slot></div>
  <div v-show="selected" class="tab-white-line"></div>
  </component>
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";

@Component({
  components: {},
})
export default class ViamTabs extends Vue {
  @Prop({ default: false }) disabled?: boolean;
  @Prop({ default: false }) selected?: boolean;
  @Prop({ default: "button" }) tag?: string;
}
</script>
<style>
  .tab-white-line {
    position: absolute;
    background-color: white;
    height: 1px;
    left: 0;
    right: 0px;
    bottom: -1px;
  }
</style>