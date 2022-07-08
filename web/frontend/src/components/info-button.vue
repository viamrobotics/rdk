<script setup lang="ts">
import Popper from "vue-popperjs";
import Icon from "./icon.vue";

interface Props {
  iconPath: string
  infoRows?: string[]
}

defineProps<Props>()

</script>

<template>
  <Popper
    trigger="hover"
    data-cy="viam-info-button-root"
    root-class="viam-info-popper"
    class="inline-flex align-middle"
    :options="{
      placement: 'top',
    }"
  >
    <div class="popper">
      <div class="viam-info-content">
        <ul>
          <li
            data-cy="viam-info-row-item"
            class="text-left"
            :key="i"
            v-for="(line, i) in infoRows ?? []"
          >
            {{ line }}
          </li>
        </ul>
      </div>

      <div class="viam-info-button-arrow"></div>
    </div>

    <button data-cy="viam-info-button-container" slot="reference">
      <Icon color="grey" :path="iconPath" />
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
