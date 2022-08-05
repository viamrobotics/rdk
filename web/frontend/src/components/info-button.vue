<script setup lang="ts">

import Popper from 'vue3-popper';
import Icon from './icon.vue';

interface Props {
  infoRows?: string[]
}

defineProps<Props>();

</script>

<template>
  <Popper
    hover
    arrow
    placement="top"
  >
    <template #content>
      <ul>
        <li
          v-for="(line, i) in infoRows ?? []"
          :key="i"
          class="text-left text-xs"
        >
          {{ line }}
        </li>
      </ul>
    </template>

    <button>
      <Icon
        color="grey"
        path="M11.016 9v-2.016h1.969v2.016h-1.969zM12 20.016q3.281 0 5.648-2.367t2.367-5.648-2.367-5.648-5.648-2.367-5.648 2.367-2.367 5.648 2.367 5.648 5.648 2.367zM12 2.016q4.125 0 7.055 2.93t2.93 7.055-2.93 7.055-7.055 2.93-7.055-2.93-2.93-7.055 2.93-7.055 7.055-2.93zM11.016 17.016v-6h1.969v6h-1.969z"
      />
    </button>
  </Popper>
</template>

<style>
  :root {
    --popper-theme-background-color: #fff;
    --popper-theme-background-color-hover: #fff;
    --popper-theme-text-color: #000;
    --popper-theme-border-color: #000;
    --popper-theme-border-width: 1px;
    --popper-theme-border-style: solid;
    --popper-theme-border-radius: 0px;
    --popper-theme-padding: 4px;
    --popper-theme-box-shadow: 0 6px 30px -6px rgba(0, 0, 0, 0.25);
  }
</style>
