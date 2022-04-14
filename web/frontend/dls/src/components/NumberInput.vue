<template>
  <div class="flex viam-number-input-root h-8">
    <input
      ref="input"
      class="viam-number-input border border-black outline-none h-full"
      type="tel"
      :value="innerValue"
      :placeholder="placeholder"
      @keydown="handleArrows"
      @input="inputEventHandler"
      @paste="pasteEventHandler"
    />
    <div
      v-show="!hideControls"
      class="
        flex
        justify-between
        flex-col
        h-full
        items-stretch
        border border-black
      "
    >
      <img
        @click="arrowClicked(increase)"
        class="viam-control-arrow cursor-pointer"
        :src="require('../assets/arrow-up.svg')"
      />
      <img
        @click="arrowClicked(decrease)"
        class="viam-control-arrow rotated-180 cursor-pointer"
        :src="require('../assets/arrow-up.svg')"
      />
    </div>
  </div>
</template>
<script lang="ts">
import { Component, Vue, Prop } from "vue-property-decorator";

const REGEXP_NUMBER = /^-?(?:[0-9]+|[0-9]+\.[0-9]*|\.[0-9]+)$/;

@Component({
  name: "NumberInput",
  model: {
    prop: "value",
    event: "input",
  },
})
export default class NumberInput extends Vue {
  @Prop({ default: -Infinity })
  public min!: number
  @Prop({ default: Infinity })
  public max!: number
  @Prop({ default: false })
  public float!: Boolean
  @Prop({ default: 1 })
  public step!: number
  @Prop({ default: false })
  public hideControls!: Boolean
  @Prop({ required: true })
  public value!: number;
  @Prop({ default: '' })
  public placeholder!: string

  get innerValue(): number {
    return this.value;
  }
  set innerValue(value: number) {
    let result = this.value;
    if (this.isNumber(value)) {
      result = this.calcValueWithRestrictions(Number(value));
    }
    this.$emit("input", result);
  }

  arrowClicked(handler: Function): void {
    //for arrows up and down working
    (this.$refs.input as HTMLInputElement).focus();
    handler();
  }
  handleArrows(event: KeyboardEvent): void {
    if (event.key === "ArrowUp") this.increase();
    else if (event.key === "ArrowDown") this.decrease();
  }

  calcValueWithRestrictions(possibleValue: number): number {
    return Math.min(this.max, Math.max(this.min, possibleValue));
  }

  inputEventHandler(event: InputEvent): void {
    const input = event.target as HTMLInputElement;
    let value = input.value.replace(/\,/g, ".");
    if (!this.float) value = input.value.replace(/\./g, "");
    if (!this.isNumber(value)) input.value = `${this.innerValue}`;
    else {
      const newValue = this.calcValueWithRestrictions(Number(value));
      input.value = `${newValue}`;
      this.innerValue = newValue;
    }
  }

  pasteEventHandler(event: ClipboardEvent): void {
    if (
      event.clipboardData &&
      !this.isNumber(event.clipboardData.getData("text"))
    )
      event.preventDefault();
  }

  changeValue(delta: number): void {
    if (Number.isNaN(this.innerValue)) this.innerValue = this.min;
    else {
      const newValue = Number((this.innerValue + delta).toFixed(2));
      this.innerValue = newValue;
    }
  }
  decrease(): void {
    this.changeValue(-this.step);
  }
  increase(): void {
    this.changeValue(+this.step);
  }
  isNumber(stringVal: any): Boolean {
    if (!this.float && !REGEXP_NUMBER.test(stringVal)) return false;

    const parsedNumber = Number(stringVal);
    return (
      Number.isFinite(parsedNumber) && REGEXP_NUMBER.test(`${parsedNumber}`)
    );
  }
}
</script>

<style scoped>
.viam-number-input {
  padding: 6px 4px;
  border-right: 0;
}
.rotated-180 {
  transform: rotate(180deg);
}
.viam-control-arrow {
  padding: 5px 4px;
}
</style>