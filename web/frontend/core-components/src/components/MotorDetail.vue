<template>
  <div class="component">
    <div class="card">
      <h2>Current Motor Status for {{ motorName }}</h2>

      <div class="details">
        <div class="detail">
          <h3>General</h3>
          <div class="details">
            <div class="detail">
              <p class="subtitle">Motor</p>
              <h3 v-if="motorStatus.on" style="color: var(--green-90)">On</h3>
              <h3 v-else>Off</h3>
            </div>
            <div class="detail">
              <p class="subtitle">Encoders Active</p>
              <h3 v-if="motorStatus.positionSupported">Yes</h3>
              <h3 v-else>No</h3>
            </div>
            <div class="detail">
              <p class="subtitle">Position</p>
              <h3>{{ motorStatus.position }}</h3>
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
            v-bind:class="['subtitle', errors.revolutions ? 'error' : '']"
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
            v-bind:class="['margin-bottom', errors.revolutions ? 'error' : '']"
            v-model="numberOfRotations"
          />
          <label
            for="rotationsPerMinuteRange"
            v-bind:class="['subtitle', errors.rpm ? 'error' : '']"
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
              v-bind:class="['margin-bottom', errors.rpm ? 'error' : '']"
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
import {
  MotorStatus,
  DirectionRelative,
  BoardMotorGoRequest,
  BoardMotorGoForRequest,
  BoardMotorGoToRequest,
} from "proto/robot_pb";

enum MotorCommandType {
  Go = "go",
  GoFor = "goFor",
  GoTo = "goTo",
}

class MotorCommand {
  type = MotorCommandType.Go;
  position = 0;
  speed = 0;
  direction: 0 | 1 | 2 = DirectionRelative.DIRECTION_RELATIVE_FORWARD;
  revolutions = 0;

  static MAX_RPM = 160;
  static STOP = new MotorCommand();

  private validateRevolutions(revolutions: number): string {
    revolutions = Number.parseFloat(revolutions.toString());
    if (Number.isNaN(revolutions)) {
      return "Input is not a number";
    } else if (revolutions < 0) {
      return "Number of revolutions cannot be less than zero";
    }
    return "";
  }

  private validateRPM(rpm: number): string {
    rpm = Number.parseFloat(rpm.toString());
    if (Number.isNaN(rpm)) {
      return "Input is not a number";
    } else if (rpm < 0) {
      return "RPM cannot be less than zero";
    } else if (rpm > MotorCommand.MAX_RPM) {
      return "RPM cannot be greater than 160";
    }
    return "";
  }

  private validatePosition(position: number): string {
    position = Number.parseFloat(position.toString());
    if (Number.isNaN(position)) {
      return "Input is not a number";
    } else if (position < 0) {
      return "Position cannot be less than zero";
    }
    return "";
  }

  validate(): { [key: string]: string } {
    let toReturn: { [key: string]: string } = {};
    switch (this.type) {
      case MotorCommandType.Go:
        toReturn = {
          rpm: this.validateRPM(this.speed),
        };
        break;
      case MotorCommandType.GoFor:
        toReturn = {
          rpm: this.validateRPM(this.speed),
          revolutions: this.validateRevolutions(this.revolutions),
        };
        break;
      case MotorCommandType.GoTo:
        toReturn = {
          rpm: this.validateRPM(this.speed),
          position: this.validatePosition(this.position),
        };
        break;
    }
    return toReturn;
  }

  asObject(): { type: string; request: unknown } {
    let req;
    switch (this.type) {
      case MotorCommandType.Go:
        req = new BoardMotorGoRequest();
        req.setDirection(this.direction);
        req.setPowerPct(this.speed / 160);
        break;
      case MotorCommandType.GoFor:
        req = new BoardMotorGoForRequest();
        req.setDirection(this.direction);
        req.setRpm(this.speed);
        req.setRevolutions(this.revolutions);
        break;
      case MotorCommandType.GoTo:
        req = new BoardMotorGoToRequest();
        req.setRpm(this.speed);
        req.setPosition(this.position);
        break;
    }
    return {
      type: this.type.toString(),
      request: req,
    };
  }
}

@Component
export default class MotorDetail extends Vue {
  @Prop() motorName!: string;
  @Prop() motorStatus!: MotorStatus.AsObject;

  motorCommand = new MotorCommand();

  get isContinuous(): boolean {
    return this.motorCommand.type === MotorCommandType.Go;
  }
  set isContinuous(continuous: boolean) {
    if (continuous) {
      this.motorCommand.type = MotorCommandType.Go;
    } else if (this.position) {
      this.motorCommand.type = MotorCommandType.GoTo;
    } else {
      this.motorCommand.type = MotorCommandType.GoFor;
    }
  }

  get isGoingForward(): boolean {
    return (
      this.motorCommand.direction ===
      DirectionRelative.DIRECTION_RELATIVE_FORWARD
    );
  }
  set isGoingForward(forward: boolean) {
    this.motorCommand.direction = forward
      ? DirectionRelative.DIRECTION_RELATIVE_FORWARD
      : DirectionRelative.DIRECTION_RELATIVE_BACKWARD;
  }

  get position(): number {
    return this.motorCommand.position;
  }
  set position(pos: number) {
    this.motorCommand.type = MotorCommandType.GoTo;
    this.motorCommand.position = pos;
  }

  get rotationsPerMinute(): number {
    return this.motorCommand.speed;
  }
  set rotationsPerMinute(rpm: number) {
    this.motorCommand.speed = rpm;
  }

  get numberOfRotations(): number {
    return this.motorCommand.revolutions;
  }
  set numberOfRotations(revolutions: number) {
    this.motorCommand.type = MotorCommandType.GoFor;
    this.motorCommand.revolutions = revolutions;
  }

  errors: { [key: string]: string } = {};
  MAX_RPM = MotorCommand.MAX_RPM;

  get command(): { [key: string]: unknown } {
    return this.motorCommand.asObject();
  }

  private validateInputs(): boolean {
    this.errors = this.motorCommand.validate();
    for (let key of Object.keys(this.errors)) {
      const error = this.errors[key];
      if (error) {
        return false;
      }
    }
    return true;
  }

  stop(): void {
    this.$emit("execute", MotorCommand.STOP.asObject());
  }

  emitCommand(): void {
    if (this.validateInputs()) {
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
