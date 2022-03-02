<template>
  <div class="component">
    <div>
      <ViamButton color="primary" group variant="primary"
        v-for="option in options"
        v-bind:key="option"
        :class="[selected === option ? 'black' : 'clear']"
        v-on:click="selectOption(option)"
        :disabled="isDisabled(option)"
      >
        <template v-slot:icon  v-if="selected === option"><font-awesome-icon icon="fa-regular fa-check-square"></font-awesome-icon></template>
        {{ option }}
      </ViamButton>
    </div>
  </div>
</template>

<script lang="ts">
import { Component, Prop, Vue, Watch } from "vue-property-decorator";
import "vue-class-component/hooks";
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome';
import ViamButton from "./Button.vue";

@Component({
  components: {
    FontAwesomeIcon,
    ViamButton
  },
})
export default class RadioButtons extends Vue {
  @Prop() options!: [string];
  @Prop() defaultOption?: string;
  @Prop() disabledOptions?: [string];

  selected: string | undefined = "";

  mounted(): void {
    this.selected = this.defaultOption;
  }

  isDisabled(option: string): boolean {
    return false;
  }

  @Watch("defaultOption")
  selectOption(option: string): void {
    this.selected = option;
    this.$emit("selectOption", option);
  }
}
</script>

<style scoped>
</style>
