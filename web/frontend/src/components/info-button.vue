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
    root-class="viam-info-popper"
    class="inline-flex align-middle"
    :options="{
      placement: 'top',
    }"
  >
    <template #content>
      <div class="popper">
        <div class="viam-info-content">
          <ul>
            <li
              v-for="(line, i) in infoRows ?? []"
              :key="i"
              data-cy="viam-info-row-item"
              class="text-left"
            >
              {{ line }}
            </li>
          </ul>
        </div>

        <div class="viam-info-button-arrow" />
      </div>
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

.viam-info-popper .popper__arrow {
  display: none;
}
.viam-info-popper .popper {
  min-width: 325px;
  border: 1px solid black;
  border-radius: 0;
  box-shadow: none;
  padding-top: 0;
  padding-bottom: 0;
}
.viam-info-popper .viam-info-content {
  position: relative;
  z-index: 3;
  padding: 3px 6px;
  background-color: white;
}
.viam-info-button-arrow {
  border-right: 1px solid black;
  border-bottom: 1px solid black;

  padding: 3px 3px;
  transform: rotate(45deg);
  background-color: white;
  position: absolute;
  right: calc(50% - 3px);
  bottom: -4px;
  z-index: 1;
}

</style>
