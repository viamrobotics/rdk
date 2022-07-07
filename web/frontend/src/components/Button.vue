<template>
  <component
    :is="props.tag"
    :ref="data.ref"
    :class="[
      'relative leading-tight font-button transition-colors duration-150 focus:outline-none shadow-sm',
      {
        'animate-pulse': props.busy,
        'text-black border border-black bg-primary hover:border-black hover:bg-gray-200 active:bg-gray-400':
          props.variant === 'primary' &&
          props.color === 'primary' &&
          !props.disabled,
        'text-white border-danger-500 bg-danger-500 hover:border-danger-600 hover:bg-danger-600 active:bg-danger-700':
          props.variant === 'primary' &&
          props.color === 'danger' &&
          !props.disabled,
        'text-white border-warning-500 bg-warning-500 hover:border-warning-600 hover:bg-warning-600 active:bg-warning-700':
          props.variant === 'primary' &&
          props.color === 'warning' &&
          !props.disabled,
        'text-white border-success-500 bg-success-500 hover:border-success-600 hover:bg-success-600 active:bg-success-700':
          props.variant === 'primary' &&
          props.color === 'success' &&
          !props.disabled,
        'text-white border-black-500 bg-black hover:border-black-600 hover:bg-black-600 active:bg-black-700':
          props.variant === 'primary' &&
          props.color === 'black' &&
          !props.disabled,
        'border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500 active:border-gray-500 dark:active:border-gray-300':
          props.variant === 'secondary' && !props.disabled,
        'border-gray-300 dark:border-gray-600 bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-400 cursor-not-allowed':
          props.disabled,
        'cursor-wait': props.loading || props.busy,
        'cursor-pointer ': !props.loading || !props.busy,
        'inline-flex items-center': $slots.icon || props.icon,
        'flex-row-reverse': props.iconRight,
        'px-4 py-1': $slots.default && props.size === 'sm',
        'px-5 py-2': $slots.default && props.size === 'base',
        'px-10 py-4': $slots.default && props.size === 'lg',
        'px-3 py-2': !$slots.default,
      },
      data.class,
      data.staticClass,
    ]"
    :style="[data.style, data.staticStyle]"
    style="height: 38px"
    :aria-busy="props.loading ? 'true' : null"
    :aria-disabled="props.tag !== 'button' && props.disabled ? 'true' : null"
    :disabled="props.disabled || props.loading"
    :type="props.tag === 'button' ? props.type : null"
    v-bind="data.attrs"
    v-on="listeners"
  >
    <div
      v-if="props.loading"
      class="absolute inset-0 flex items-center justify-center w-full"
    >
      &#8203;
      <div class="rounded-full h-2 w-2 mx-1 bg-current animate-pulse" />
      <div
        class="rounded-full h-2 w-2 mx-1 bg-current animate-pulse animation-delay-300"
      />
      <div
        class="rounded-full h-2 w-2 mx-1 bg-current animate-pulse animation-delay-600"
      />
      &#8203;
    </div>

    <span
      v-if="$slots.icon"
      class=""
      :class="[
        '',
        {
          'opacity-0': props.loading,
          'mr-1': $slots.default && !props.iconRight,
          'ml-1': $slots.default && props.iconRight,
        },
      ]"
    >
      <slot name="icon" />
    </span>
    <span :class="{ 'opacity-0': props.loading }">
      <slot />
    </span>
    <span v-if="!$slots.default">&#8203;</span>
  </component>
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";

@Component
export default class ViamButton extends Vue {
  @Prop({ default: false })
  busy!: boolean;
  @Prop({ default: "primary" })
  color!: string;
  @Prop({ default: false })
  disabled!: boolean;
  @Prop({ default: false })
  group!: boolean | string;
  @Prop({ default: null })
  icon!: string;
  @Prop({ default: false })
  iconRight!: boolean;
  @Prop({ default: false })
  loading!: boolean;
  @Prop({ default: "base" })
  size!: string;
  @Prop({ default: "button" })
  tag!: string;
  @Prop({ default: "button" })
  type!: string;
  @Prop({ default: "primary" })
  variant!: string;
}
</script>
