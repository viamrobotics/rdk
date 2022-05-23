<template>
  <div class="flex viam-number-input-root h-8">
    <input
      :id="id"
      ref="input"
      class="viam-number-input border-black outline-none h-full w-full"
      type="tel"
      :value="innerValue"
      :placeholder="placeholder"
      :readonly="readonly"
      @keydown="handleArrows"
      @input="inputEventHandler"
      @paste="pasteEventHandler"
      :class="{
        'border-r': readonly,
        'text-center': readonly,
        'text-xs': this.small,
      }"
    />
    <div
      v-show="!readonly"
      class="flex justify-between flex-col h-full items-stretch border border-black"
    >
      <ViamIcon
        @click.native="arrowClicked(increase)"
        class="arrow-icon cursor-pointer"
        :path="mdiChevronUp"
      ></ViamIcon>
      <ViamIcon
        @click.native="arrowClicked(decrease)"
        class="arrow-icon cursor-pointer"
        :path="mdiChevronDown"
      ></ViamIcon>
    </div>
  </div>
</template>
<script lang="ts">
import { Component, Vue, Prop } from "vue-property-decorator";
import { mdiChevronDown, mdiChevronUp } from "@mdi/js";
import ViamIcon from "./ViamIcon.vue";
const REGEXP_NUMBER = /^-?(?:[0-9]+|[0-9]+\.[0-9]*|\.[0-9]+)$/;

@Component({
  name: "NumberInput",
  model: {
    prop: "value",
    event: "input",
  },
  components: {
    ViamIcon,
  },
})
export default class NumberInput extends Vue {
  @Prop({ default: -Infinity })
  public min!: number;
  @Prop({ default: Infinity })
  public max!: number;
  @Prop({ default: false })
  public float!: boolean;
  @Prop({ default: 1 })
  public step!: number;
  @Prop({ default: false })
  public readonly!: boolean;
  @Prop({ required: true })
  public value!: number;
  @Prop({ default: "" })
  public placeholder!: string;
  @Prop({ default: false })
  public small!: boolean;
  @Prop({ default: "DefaultId" })
  id!: string;

  mdiChevronDown = mdiChevronDown;
  mdiChevronUp = mdiChevronUp;

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

  arrowClicked(handler: () => void): void {
    //for arrows up and down working
    (this.$refs.input as HTMLInputElement).focus();
    handler();
  }
  handleArrows(event: KeyboardEvent): void {
    if (this.readonly) return;
    if (event.key === "ArrowUp") this.increase();
    else if (event.key === "ArrowDown") this.decrease();
  }

  calcValueWithRestrictions(possibleValue: number): number {
    return Math.min(this.max, Math.max(this.min, possibleValue));
  }

  inputEventHandler(event: InputEvent): void {
    const input = event.target as HTMLInputElement;
    let value = input.value.replace(/,/g, ".");
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
  // eslint-disable-next-line
  isNumber(stringVal: any): boolean {
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
  border-left-width: 1px;
  border-top-width: 1px;
  border-bottom-width: 1px;
}
</style>
