<template>
  <v-collapse
    title="SLAM"
    class="slam"
  >
    <div class="h-auto border-x border-b border-black p-2">
      <div class="container mx-auto">
        <div class="pt-4">
          <div class="flex items-center gap-2">
            <v-switch
              id="showImage"
              :value="showImage ? 'on' : 'off'"
              @input="toggleImageExpand()"
            />
            <span class="pr-2">View SLAM Map (JPEG)</span>
          </div>
          <div class="float-right pb-4">
            <div class="flex">
              <div
                v-if="showImage"
                class="w-64"
              >
                <p class="font-label mb-1 text-gray-800 dark:text-gray-200">
                  Refresh frequency
                </p>
                <div class="relative">
                  <select
                    v-model="selectedImageValue"
                    class="m-0 w-full appearance-none border border-solid border-black bg-white bg-clip-padding px-3 py-1.5 text-xs font-normal text-gray-700 focus:outline-none"
                    aria-label="Default select example"
                    @change="selectSLAMImageRefreshFrequency()"
                  >
                    <option value="manual">
                      Manual Refresh
                    </option>
                    <option value="30">
                      Every 30 seconds
                    </option>
                    <option value="10">
                      Every 10 seconds
                    </option>
                    <option value="5">
                      Every 5 seconds
                    </option>
                  </select>
                  <div
                    class="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2"
                  >
                    <svg
                      class="h-4 w-4 stroke-2 text-gray-700 dark:text-gray-300"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      stroke-linejoin="round"
                      stroke-linecap="round"
                      fill="none"
                    >
                      <path d="M18 16L12 22L6 16" />
                    </svg>
                  </div>
                </div>
              </div>
              <div class="px-2 pt-7">
                <v-button
                  v-if="showImage"
                  icon="refresh"
                  label="Refresh"
                  @click="refreshImageMap()"
                />
              </div>
            </div>
          </div>
          <img
              v-if="showImage"
              :src="imageMap"
              width="500"
              height="500"
            >
        </div>
        <div class="pt-4">
          <div class="flex items-center gap-2">
            <v-switch
              :value="showPCD ? 'on' : 'off'"
              @input="togglePCDExpand()"
            />
            <span class="pr-2">View SLAM Map (PCD)</span>
          </div>
          <div class="float-right pb-4">
            <div class="flex">
              <div
                v-if="showPCD"
                class="w-64"
              >
              <p class="font-label mb-1 text-gray-800 dark:text-gray-200">
                  Refresh frequency
              </p>
              <div class="relative">
                <select
                  v-model="selectedPCDValue"
                  class="m-0 w-full appearance-none border border-solid border-black bg-white bg-clip-padding px-3 py-1.5 text-xs font-normal text-gray-700 focus:outline-none"
                  aria-label="Default select example"
                  @change="selectSLAMPCDRefreshFrequency()"
                >
                  <option value="manual">
                    Manual Refresh
                  </option>
                  <option value="30">
                    Every 30 seconds
                  </option>
                  <option value="10">
                    Every 10 seconds
                  </option>
                  <option value="5">
                    Every 5 seconds
                  </option>
                </select>
                <div
                  class="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2"
                >
                  <svg
                    class="h-4 w-4 stroke-2 text-gray-700 dark:text-gray-300"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-linejoin="round"
                    stroke-linecap="round"
                    fill="none"
                  >
                    <path d="M18 16L12 22L6 16" />
                  </svg>
                </div>
              </div>
            </div>
            <div class="px-2 pt-7">
              <v-button
                v-if="showPCD"
                icon="refresh"
                label="Refresh"
                @click="refreshPCDMap"
              />
            </div>
          </div>
        </div>
        <div
          v-if="showPCD"
          id="pcd"
          width="400"
          height="400"
        />
        </div>
      </div>
    </div>
  </v-collapse>
</template>

<script setup lang="ts">

import { ref } from 'vue';

interface Props {
  imageMap?: string
}

interface Emits {
  (event: 'update-slam-image-refresh-frequency', value: string): void
  (event: 'update-slam-pcd-refresh-frequency', value: string, load: boolean): void
}

defineProps<Props>();

const emit = defineEmits<Emits>();

const showImage = ref(false);
const showPCD = ref(false);
const selectedImageValue = ref('manual');
const selectedPCDValue = ref('manual');

const toggleImageExpand = () => {
  showImage.value = !showImage.value;
  if (showImage.value) {
    emit('update-slam-image-refresh-frequency', selectedImageValue.value);
  } else {
    emit('update-slam-image-refresh-frequency', "off");
  }
};

const togglePCDExpand = () => {
  showPCD.value = !showPCD.value;
  if (showPCD.value) {
    emit('update-slam-pcd-refresh-frequency', selectedPCDValue.value, true);
  } else {
    emit('update-slam-pcd-refresh-frequency', "off", false);
  }
};

const selectSLAMImageRefreshFrequency = () => {
  emit('update-slam-image-refresh-frequency', selectedImageValue.value);
};

const selectSLAMPCDRefreshFrequency = () => {
  emit('update-slam-pcd-refresh-frequency', selectedPCDValue.value, false);
};

const refreshImageMap = () => {
  emit('update-slam-image-refresh-frequency', selectedImageValue.value);
};

const refreshPCDMap = () => {
  emit('update-slam-pcd-refresh-frequency', selectedPCDValue.value, false);
};

</script>
