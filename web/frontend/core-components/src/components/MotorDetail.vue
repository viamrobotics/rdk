<template>
  <div class="component">
    <div class="card">
      <div class="header">
        <h2>{{ motorName }} Motor</h2>
        <span v-if="motorStatus.on" class="pill green">Running</span>
        <span v-else class="pill">Idle</span>
      </div>

      <!-- <div style="border: 1px solid gray;">
        <h4 style="margin: 0;">Position control coming soon</h4>
        <div v-if="!motorStatus.positionSupported" class="row">
          <div class="column">
            <label for="positionInput" class="subtitle">Position</label>
            <input name="positionInput" id="positionInput" type="number" disabled />
          </div>
          <button class="clear" disabled>
            <i class="fas fa-home"></i>
            Set Home
          </button>
          <button class="clear" disabled>
            <i class="fas fa-crosshairs"></i>
            Go to Home
          </button>
        </div>
      </div> -->

      <div class="row" v-if="motorStatus.positionSupported">
        <div class="column">
          <h2>{{ motorStatus.position }}</h2>
          <p class="subtitle">Position</p>
        </div>
      </div>

      <div class="row">
        <div class="column">
          <p class="subtitle">Type of Rotation</p>
          <RadioButtons
            :options="['Continuous', 'Discrete']"
            :defaultOption="isContinuous ? 'Continuous' : 'Discrete'"
            :disabledOptions="motorStatus.positionSupported ? [] : ['Discrete']"
            v-on:selectOption="isContinuous = $event === 'Continuous'"
          />
        </div>
        <div class="column">
          <label
            for="numberOfRotations"
            v-bind:class="['subtitle', errors.revolutions ? 'error' : '']"
          >
            Number of Rotations
            {{ errors.revolutions ? " - " + errors.revolutions : "" }}
          </label>
          <input
            id="numberOfRotations"
            name="numberOfRotations"
            type="text"
            placeholder="Enter a number"
            min="0"
            :disabled="isContinuous"
            v-bind:class="['margin-bottom', errors.revolutions ? 'error' : '']"
            style="max-width: 128px"
            v-model="numberOfRotations"
          />
        </div>
        <div class="column">
          <p class="subtitle">Direction of Rotation</p>
          <RadioButtons
            :options="['Backward', 'Forward']"
            :defaultOption="isGoingForward ? 'Forward' : 'Backward'"
            v-on:selectOption="isGoingForward = $event === 'Forward'"
          />
        </div>
      </div>
      <div class="row">
        <div class="column">
          <label
            for="speedRange"
            v-bind:class="['subtitle', errors.speed ? 'error' : '']"
          >
            Mode
            {{ errors.speed ? " - " + errors.speed : "" }}
          </label>
          <div class="row" style="align-items: center">
            <RadioButtons
              :options="['Power', 'RPM']"
              :defaultOption="isContinuous ? 'Power' : 'RPM'"
              :disabledOptions="isContinuous ? ['RPM'] : ['Power']"
              style="flex-shrink: 0"
            />
            <input
              id="speedRange"
              name="speedRange"
              type="range"
              v-model="speed"
              min="0"
              v-bind:max="motorStatus.positionSupported ? MAX_RPM : 100"
              style="flex-shrink: 0"
            />
            <input
              name="speedFinite"
              id="speedFinite"
              type="text"
              v-model="speed"
              min="0"
              v-bind:max="motorStatus.positionSupported ? MAX_RPM : 100"
              v-bind:class="['margin-bottom', errors.speed ? 'error' : '']"
              style="min-width: 48px; max-width: 48px; flex-shrink: 0"
            />
          </div>
        </div>
      </div>

      <div class="row" style="justify-content: flex-end">
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
  MotorGoRequest,
  MotorGoForRequest,
  MotorGoToRequest,
} from "proto/robot_pb";
import RadioButtons from "./RadioButtons.vue";

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
  static get STOP(): MotorCommand {
    const cmd = new MotorCommand();
    cmd.direction = DirectionRelative.DIRECTION_RELATIVE_UNSPECIFIED;
    return cmd;
  }

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

  private validatePower(power: number): string {
    power = Number.parseFloat(power.toString());
    if (Number.isNaN(power)) {
      return "Input is not a number";
    } else if (power < 0) {
      return "Power cannot be less than zero";
    } else if (power > 100) {
      return "Power cannot be greater than 100%";
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
          speed: this.validatePower(this.speed),
        };
        break;
      case MotorCommandType.GoFor:
        toReturn = {
          speed: this.validateRPM(this.speed),
          revolutions: this.validateRevolutions(this.revolutions),
        };
        break;
      case MotorCommandType.GoTo:
        toReturn = {
          speed: this.validateRPM(this.speed),
          position: this.validatePosition(this.position),
        };
        break;
    }
    return toReturn;
  }

  asObject(): { type: string; request: MotorGoRequest | MotorGoForRequest | MotorGoToRequest } {
    let req;
    switch (this.type) {
      case MotorCommandType.Go:
        req = new MotorGoRequest();
        req.setDirection(this.direction);
        req.setPowerPct(this.speed / 100);
        break;
      case MotorCommandType.GoFor:
        req = new MotorGoForRequest();
        req.setDirection(this.direction);
        req.setRpm(this.speed);
        req.setRevolutions(this.revolutions);
        break;
      case MotorCommandType.GoTo:
        req = new MotorGoToRequest();
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

@Component({
  components: {
    RadioButtons,
  },
})
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

  get speed(): number {
    return this.motorCommand.speed;
  }
  set speed(v: number) {
    this.motorCommand.speed = v;
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
      const command = this.motorCommand.asObject()
      console.log(command);
      const req = command['request'] as MotorGoRequest;
      this.$emit("execute", command);
    }
  }
}
</script>

<style scoped>
p,
h2,
h3 {
  margin: 0;
}

.header {
  display: flex;
  flex-direction: row;
  align-items: center;
  align-content: center;
  gap: 8px;
}

.row {
  display: flex;
  flex-direction: row;
  margin-right: 12px;
  align-items: flex-end;
  gap: 8px;
  margin-bottom: 12px;
}

.subtitle {
  color: var(--black-70);
}

.column {
  display: flex;
  flex-direction: column;
  margin-left: 0px;
}

.margin-bottom {
  margin-bottom: 32px;
}
</style>
