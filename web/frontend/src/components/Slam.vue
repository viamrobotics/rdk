<template>
  <v-collapse title="SLAM" class="slam">
    <div class="flex mb-4">
      <table class="border border-slate-300 mr-4">
        <tr>
          <th colspan="1" class="border border-slate-300">
            <h5>
              View SLAM Map (JPEG)
              <v-button
                group
                label="Refresh"
                @click="emit('refresh-image-map')"
              />
            </h5>
          </th>
        </tr>
        <tr>
          <th colspan="1" class="border border-slate-300">
            <img
              v-if="imageMap"
              :src="imageMap"
              width="500"
              height="500"
            />
          </th>
        </tr>
      </table>
      <table class="border border-slate-300 mr-4">
        <tr>
          <th colspan="1" class="border border-slate-300">
            <h5>
              View SLAM Map (PCD)
              <v-button
                group
                label="Refresh"
                @click="emit('refresh-pcd-map')"
              />
            </h5>
          </th>
        </tr>
        <tr>
          <th colspan="1" class="border border-slate-300">
            <div id="pcd" width="400" height="400" />
          </th>
        </tr>
      </table>
    </div>
  </v-collapse>
</template>

<script setup lang="ts">

interface Props {
  imageMap?: string
}

interface Emits {
  (event: 'refresh-image-map'): void
  (event: 'refresh-pcd-map'): void
}

defineProps<Props>()
const emit = defineEmits<Emits>()

</script>
