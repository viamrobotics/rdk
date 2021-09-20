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
    </div>

    <div class="card">
      <h2>Single Motor Control</h2>

      <div class="details">
        <div class="detail">
          <p class="subtitle">Type of Rotation</p>
          <div class="details margin-bottom">
            <button
              v-bind:class="[isContinuous ? 'blue' : 'clear']"
              v-on:click="isContinuous = true"
            >
              Continuous
            </button>
            <button
              v-bind:class="[!isContinuous ? 'blue' : 'clear']"
              v-on:click="isContinuous = false"
            >
              Defined
            </button>
          </div>
          <p class="subtitle">Direction of Rotation</p>
          <div class="details margin-bottom">
            <button
              v-bind:class="[isGoingForward ? 'blue' : 'clear']"
              v-on:click="isGoingForward = true"
            >
              Forward
            </button>
            <button
              v-bind:class="[!isGoingForward ? 'blue' : 'clear']"
              v-on:click="isGoingForward = false"
            >
              Backward
            </button>
          </div>
        </div>
        <div class="detail" style="flex-grow: 1">
          <label
            for="numberOfRotations"
            v-bind:class="['subtitle', numberOfRotationsError ? 'error' : '']"
          >
            Number of Rotations
          </label>
          <input
            id="numberOfRotations"
            name="numberOfRotations"
            type="text"
            placeholder="Enter a number"
            min="0"
            :disabled="isContinuous"
            v-bind:class="[
              'margin-bottom',
              numberOfRotationsError ? 'error' : '',
            ]"
            v-model="numberOfRotations"
          />
          <label
            for="rotationsPerMinuteRange"
            v-bind:class="['subtitle', rotationsPerMinuteError ? 'error' : '']"
          >
            Rotations Per Minute
          </label>
          <div class="details">
            <input
              id="rotationsPerMinuteRange"
              name="rotationsPerMinuteRange"
              type="range"
              v-model="rotationsPerMinute"
              min="0"
              v-bind:max="MAX_RPM"
            />
            <input
              name="rotationsPerMinuteFinite"
              id="rotationsPerMinuteFinite"
              type="text"
              v-model="rotationsPerMinute"
              min="0"
              v-bind:max="MAX_RPM"
              v-bind:class="[
                'margin-bottom',
                rotationsPerMinuteError ? 'error' : '',
              ]"
            />
          </div>
        </div>
      </div>

      <div class="details" style="justify-content: flex-end">
        <button class="red" v-on:click="stop">
          <i class="far fa-times-circle"></i>
          STOP
        </button>
        <button class="green" v-on:click="emitCommand">
          <i class="fas fa-play"></i>
          RUN
        </button>
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
  numberOfRotations = "";
  numberOfRotationsError = false;
  rotationsPerMinute = "0";
  rotationsPerMinuteError = false;
  MAX_RPM = 160;

  get command(): { [key: string]: unknown } {
    let cmd: { [key: string]: unknown } = {
      d: this.isGoingForward ? "forward" : "backward",
      s: this.isContinuous ? Number.parseFloat(this.rotationsPerMinute)/this.MAX_RPM : Number.parseFloat(this.rotationsPerMinute),
    };
    if (!this.isContinuous) {
      cmd["r"] = Number.parseFloat(this.numberOfRotations);
    }
    return cmd;
  }

  private clearErrors() {
    this.numberOfRotationsError = false;
    this.rotationsPerMinuteError = false;
  }

  private validateInputs(): boolean {
    let hasErrors = false;
    this.clearErrors();
    if (!this.isContinuous) {
      const numberOfRotations = Number.parseFloat(this.numberOfRotations);
      if (Number.isNaN(numberOfRotations)) {
        this.numberOfRotationsError = true;
        hasErrors = true;
      } else if (numberOfRotations < 0) {
        this.numberOfRotationsError = true;
        hasErrors = true;
      }
    }
    const rotationsPerMinute = Number.parseFloat(this.rotationsPerMinute);
    if (Number.isNaN(rotationsPerMinute)) {
      this.rotationsPerMinuteError = true;
      hasErrors = true;
    } else if (rotationsPerMinute < 0) {
      this.rotationsPerMinuteError = true;
      hasErrors = true;
    } else if (rotationsPerMinute > this.MAX_RPM) {
      this.rotationsPerMinuteError = true;
      hasErrors = true;
    }
    return !hasErrors;
  }

  stop(): void {
    this.$emit("execute", { d: "none", s: 0 });
  }

  emitCommand(): void {
    if (this.validateInputs()) {
      console.log(this.command);
      this.$emit("execute", this.command);
    }
  }
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

.margin-bottom {
  margin-bottom: 32px;
}
</style>
