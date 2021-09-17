<template>
  <div class="component">
    <div class="card">
      <h2>Current Motor Status for {{ motor[0] }}</h2>

      <div class="details">
        <div class="detail">
          <h3>General</h3>
          <div class="details">
            <div class="detail">
              <p class="subtitle">Motor</p>
              <h3 v-if="motor[1].on" style="color: var(--green-90)">On</h3>
              <h3 v-else>Off</h3>
            </div>
            <div class="detail">
              <p class="subtitle">Encoders Active</p>
              <h3 v-if="motor[1].positionSupported">Yes</h3>
              <h3 v-else>No</h3>
            </div>
            <div class="detail">
              <p class="subtitle">Position</p>
              <h3>{{ motor[1].position }}</h3>
            </div>
          </div>
        </div>
      </div>

      <button class="button" v-on:click="$emit('execute')">RUN</button>
    </div>

    <div class="card">
      <h2>Single Motor Control</h2>

      <div class="details">
        <div class="detail">
          <p class="subtitle">Type of Rotation</p>
          <div class="details">
            <button
              class="blue"
              :disabled="isContinuous"
              v-on:click="isContinuous = !isContinuous"
            >
              Continuous
            </button>
            <button
              :disabled="!isContinuous"
              v-on:click="isContinuous = !isContinuous"
            >
              Defined
            </button>
          </div>
          <p class="subtitle">Direction of Rotation</p>
        </div>
        <div class="detail">
          <input type="number" placeholder="Number of Rotations" />
          <p class="subtitle">Rotations Per Minute</p>
          <input type="range" />
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";

@Component
export default class MotorDetail extends Vue {
  @Prop() private motor!: [string, { [key: string]: Record<string, unknown> }];

  isContinuous = true;
  isGoingForward = true;
}
</script>

<style scoped>
h2 {
  margin: 0px;
}
.details {
  display: flex;
  flex-direction: row;
  margin-right: 12px;
}

.details p,
.details h3 {
  margin: 0;
}

.details .subtitle {
  color: var(--black-70);
}

.detail {
  display: flex;
  flex-direction: column;
  margin: 12px;
  margin-left: 0px;
}
</style>
