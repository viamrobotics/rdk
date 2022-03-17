<template functional>
  <component
    :is="props.tag"
    :ref="data.ref"
    class="rounded-full inline-block leading-tight"
    :class="[
      {
        'bg-blue-200 text-blue-900':
          props.variant === 'fill' && props.color === 'blue',
        'bg-gray-200 text-gray-900 dark:bg-gray-700 dark:text-gray-100':
          props.variant === 'fill' && props.color === 'gray',
        'bg-gray-200 text-gray-900':
          props.variant === 'fill' && props.color === 'gray',
        'bg-green-200 text-green-900':
          props.variant === 'fill' && props.color === 'green',
        'bg-orange-200 text-orange-900':
          props.variant === 'fill' && props.color === 'orange',
        'bg-primary-200 text-primary-900':
          props.variant === 'fill' && props.color === 'primary',
        'bg-red-200 text-red-900':
          props.variant === 'fill' && props.color === 'red',
        'bg-yellow-200 text-yellow-900':
          props.variant === 'fill' && props.color === 'yellow',
        border: props.variant === 'border',
        'text-blue-900 dark:text-blue-200 border-blue-300 dark:border-blue-500':
          props.variant === 'border' && props.color === 'blue',
        'text-gray-900 dark:text-gray-200 border-gray-300 dark:border-gray-500':
          props.variant === 'border' && props.color === 'gray',
        'text-green-900 dark:text-green-200 border-green-300 dark:border-green-500':
          props.variant === 'border' && props.color === 'green',
        'text-orange-900 dark:text-orange-200 border-orange-300 dark:border-orange-500':
          props.variant === 'border' && props.color === 'orange',
        'text-primary-900 dark:text-primary-200 border-primary-300 dark:border-primary-500':
          props.variant === 'border' && props.color === 'primary',
        'text-red-900 dark:text-red-200 border-red-300 dark:border-red-500':
          props.variant === 'border' && props.color === 'red',
        'text-yellow-900 dark:text-yellow-200 border-yellow-300 dark:border-yellow-500':
          props.variant === 'border' && props.color === 'yellow',
        'text-sm px-2 py-px': props.size === 'sm',
        'px-3': props.size === 'base',
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

@Component({
  components: {},
})
export default class ViamBadge extends Vue {
  @Prop({ default: "primary" })
  color!: string;
  @Prop({ default: "base" })
  size!: string;
  @Prop({ default: "span" })
  tag!: string;
  @Prop({ default: "fill" })
  variant!: string;
}
</script>
