<template>
  <div class="component">
    <div class="radio-buttons">
      <button
        v-for="option in options"
        v-bind:key="option"
        :class="[selected === option ? 'black' : 'clear']"
        v-on:click="selectOption(option)"
        :disabled="isDisabled(option)"
      >
        <i v-if="selected === option" class="fas fa-check"></i>
        {{ option }}
      </button>
    </div>
  </div>
</template>

<script lang="ts">
import { Component, Prop, Vue, Watch } from "vue-property-decorator";
import "vue-class-component/hooks";

@Component
export default class MotorDetail extends Vue {
  @Prop() options!: [string];
  @Prop() defaultOption?: string;
  @Prop() disabledOptions?: [string];

  selected: string | undefined = "";

  mounted(): void {
    this.selected = this.defaultOption;
  }

  isDisabled(option: string): boolean {
    return !!this.disabledOptions?.includes(option);
  }

  @Watch("defaultOption")
  selectOption(option: string): void {
    this.selected = option;
    this.$emit("selectOption", option);
  }
}
</script>

<style scoped>
.radio-buttons {
  display: flex;
  flex-direction: row;
}
.radio-buttons button {
  margin: 0;
}
</style>
