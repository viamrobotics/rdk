<template>
  <component
    :is="tag"
    :class="[
      'flex-grow transition-colors duration-150 ease-in-out px-6 py-0.1 focus:outline-none',
      {
        'border-t border-l border-r border-black bg-white dark:bg-gray-600 text-gray-900 dark:text-gray-50 shadow-sm':
          selected,
        'border-r border-grey cursor-pointer hover:text-gray-700 dark:hover:text-gray-200 focus:ring-2 focus:ring-white dark:focus:ring-gray-700':
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
    <slot />
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
