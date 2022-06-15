<template>
  <label
    :class="[
      !disabled ? 'cursor-pointer' : 'cursor-not-allowed',
      $slots.default ? 'inline-flex' : 'inline-block',
      {
        'items-center': centered && $slots.default,
        'flex-row-reverse': reversed && $slots.default,
      },
    ]"
  >
    <div
      class="rounded-full transition-colors duration-300"
      :class="[
        {
          'flex-none': $slots.default,
          'w-6 h-3': size === 'sm',
          'my-2': size === 'sm' && $slots.default,
          'w-10 h-6 p-1': size === 'base',
          'bg-gray-300 dark:bg-gray-600': disabled && !checked,
          'bg-gray-400 dark:bg-gray-400': disabled && checked,
          'bg-gray-200 dark:bg-gray-600': !disabled && !checked,
          'bg-green-600': !disabled && checked,
        },
      ]"
    >
      <input
        :id="id"
        v-model="checked"
        :aria-checked="checked ? 'true' : 'false'"
        :aria-disabled="disabled ? 'true' : null"
        :disabled="disabled"
        :name="name"
        :required="required"
        type="checkbox"
        class="hidden"
      />

      <div
        class="rounded-full shadow transform transition duration-300"
        :class="[
          {
            'w-3 h-3': size === 'sm',
            'w-4 h-4': size === 'base',
            'translate-x-full': checked,
            'bg-white dark:bg-white-200': disabled,
            'bg-white': !disabled && !checked,
            'bg-white': !disabled && checked,
          },
        ]"
      />
    </div>
    <div
      v-if="$slots.default"
      :class="['flex-grow', reversed ? 'pr-2' : 'pl-2']"
    >
      <slot />
    </div>
  </label>
</template>

<script lang="ts">
import { Component, Prop, Vue, Model } from 'vue-property-decorator';
import 'vue-class-component/hooks';

@Component
export default class ViamSwitch extends Vue {
  @Prop({ default: false }) centered?: boolean;
  @Prop({ default: false }) disabled?: boolean;
  @Prop({ default: null }) id?: string;
  @Prop({ default: null }) name?: string;
  @Prop({ default: false }) required?: boolean;
  @Prop({ default: false }) reversed?: boolean;
  @Prop({ default: 'base' }) size?: string;

  @Model('change', { type: Boolean }) option!: boolean;

  get checked(): boolean {
    return this.option;
  }

  set checked(val: boolean) {
    this.$emit('change', val);
  }
}
</script>
