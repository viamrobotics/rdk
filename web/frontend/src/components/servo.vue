<script setup lang="ts">

interface Props {
  servoName: string
  servoAngle: string
}

interface Emits {
  (event: 'motor-run'): void
  (event: 'servo-stop'): void
  (event: 'servo-move', amount: number): void
}

defineProps<Props>();
const emit = defineEmits<Emits>();

const servoStop = () => {
  emit('servo-stop');
};

</script>

<template>
  <div>
    <v-collapse :title="servoName">
      <v-button
        slot="header"
        label="STOP"
        icon="stop-circle"
        variant="danger"
        @click="servoStop"
      />
      <div class="border border-t-0 border-black p-4">
        <h3 class="mb-1 text-sm">
          Angle: {{ servoAngle }}
        </h3>
           
        <div class="flex gap-1.5">
          <v-button
            label="-10"
            @click="$emit('servo-move', -10)"
          />
          <v-button
            label="-1"
            @click="$emit('servo-move', -1)"
          />
          <v-button
            label="1"
            @click="$emit('servo-move', 1)"
          />
          <v-button
            label="10"
            @click="$emit('servo-move', 10)"
          />
        </div>
      </div>
    </v-collapse>
  </div>
</template>
