<template>
  <div class="component">
    <div class="card">
      <div class="row" style="margin-right: 0; align-items: center">
        <div class="header">
          <h2>{{ motorName }} Motor</h2>
          <span v-if="motorStatus.isOn" class="pill green">Running</span>
          <span v-else class="pill">Idle</span>
        </div>
        <div class="column" v-if="motorStatus.positionReporting">
          <h3 style="line-height: 0.65">{{ motorStatus.position }}</h3>
          <p class="subtitle">Position</p>
        </div>
        <div
          class="row"
          style="justify-content: flex-end; flex-grow: 1; margin-right: 0"
        >
          <button class="red" v-on:click="stop" style="align-self: flex-end">
            <i class="far fa-times-circle"></i>
            STOP
          </button>
          <button
            class="green"
            v-on:click="emitCommand"
            style="align-self: flex-end"
          >
            <i class="fas fa-play"></i>
            RUN
          </button>
        </div>
      </div>
      <div class="row" style="justify-content: space-between">
        <div class="row">
          <div class="column">
            <p class="subtitle">Type of Rotation</p>
            <RadioButtons
              :options="['Continuous', 'Discrete']"
              :defaultOption="isContinuous ? 'Continuous' : 'Discrete'"
              :disabledOptions="
                motorStatus.positionReporting ? [] : ['Discrete']
              "
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
              v-bind:class="[
                'margin-bottom',
                errors.revolutions ? 'error' : '',
              ]"
              style="max-width: 128px"
              v-model="numberOfRotations"
            />
          </div>
        </div>
        <div class="column">
          <p class="subtitle">Direction of Rotation</p>
          <RadioButtons
            :options="['Forward', 'Backward']"
            :defaultOption="isGoingForward ? 'Forward' : 'Backward'"
            v-on:selectOption="isGoingForward = $event === 'Forward'"
          />
        </div>
        <div class="row">
          <div class="column">
            <label class="subtitle">Mode</label>
            <RadioButtons
              :options="['Power', 'RPM']"
              :defaultOption="isContinuous ? 'Power' : 'RPM'"
              :disabledOptions="isContinuous ? ['RPM'] : ['Power']"
            />
          </div>
          <div class="column">
            <label
              for="speedFinite"
              v-bind:class="['subtitle', errors.speed ? 'error' : '']"
            >
              {{ isContinuous ? "Power" : "RPM" }}
              {{ errors.speed ? " - " + errors.speed : "" }}
            </label>
            <div class="input-group">
              <input
                name="speedFinite"
                id="speedFinite"
                type="text"
                v-model="speed"
                min="0"
                v-bind:max="motorStatus.positionReporting ? '' : 100"
                v-bind:class="['margin-bottom', errors.speed ? 'error' : '']"
                style="width: 48px"
              />
              <span class="input-post">{{ isContinuous ? "%" : "RPM" }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";
import {
  SetPowerRequest,
  GoForRequest,
  GoToRequest,
  Status,
} from "proto/api/component/motor/v1/motor_pb";
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
  direction: -1 | 1 = 1;
  revolutions = 0;

  static get STOP(): MotorCommand {
    const cmd = new MotorCommand();
    return cmd;
  }

  private validateRevolutions(revolutions: number): string {
    revolutions = Number.parseFloat(revolutions.toString());
    if (Number.isNaN(revolutions)) {
      return "Input is not a number";
    }
    return "";
  }

  private validateRPM(rpm: number): string {
    rpm = Number.parseFloat(rpm.toString());
    if (Number.isNaN(rpm)) {
      return "Input is not a number";
    }
    return "";
  }

  private validatePower(power: number): string {
    power = Number.parseFloat(power.toString());
    if (Number.isNaN(power)) {
      return "Input is not a number";
    } else if (power > 100) {
      return "Power cannot be greater than 100%";
    } else if (power < -100) {
      return "Power cannot be less than -100%";
    }
    return "";
  }

  private validatePosition(position: number): string {
    position = Number.parseFloat(position.toString());
    if (Number.isNaN(position)) {
      return "Input is not a number";
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

  asObject(): {
    type: string;
    request: SetPowerRequest | GoForRequest | GoToRequest;
  } {
    let req;
    switch (this.type) {
      case MotorCommandType.Go:
        req = new SetPowerRequest();
        req.setPowerPct((this.speed * this.direction) / 100);
        break;
      case MotorCommandType.GoFor:
        req = new GoForRequest();
        req.setRpm(this.speed * this.direction);
        req.setRevolutions(this.revolutions);
        break;
      case MotorCommandType.GoTo:
        req = new GoToRequest();
        req.setRpm(this.speed);
        req.setPositionRevolutions(this.position);
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
  @Prop() motorStatus!: Status.AsObject;

  motorCommand = new MotorCommand();

  mounted(): void {
    if (this.motorStatus.positionReporting) {
      this.motorCommand.type = MotorCommandType.GoFor;
      this.motorCommand.speed = 10;
      this.motorCommand.revolutions = 1;
    }
  }
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
    return this.motorCommand.direction === 1;
  }

  set isGoingForward(forward: boolean) {
    this.motorCommand.direction = forward ? 1 : -1;
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
      const command = this.motorCommand.asObject();
      console.log(command);
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
