<template>
  <div>
    <div class="inline-flex">
      <ViamButton
        group
        variant="primary"
        v-for="option in options"
        v-bind:key="option"
        :color="selected === option ? 'black' : 'primary'"
        v-on:click="selectOption(option)"
        :disabled="isDisabled(option)"
      >
        <template v-slot:icon v-if="selected === option"
          ><ViamIcon
            :color="selected === option ? 'white' : 'black'"
            :path="mdiCheck"
            >Check</ViamIcon
          ></template
        >
        {{ option }}
      </ViamButton>
    </div>
  </div>
</template>

<script lang="ts">
import { Component, Prop, Vue, Watch } from "vue-property-decorator";
import "vue-class-component/hooks";
import { FontAwesomeIcon } from "@fortawesome/vue-fontawesome";
import { mdiCheck } from "@mdi/js";
import ViamButton from "./Button.vue";
import ViamIcon from "./ViamIcon.vue";
@Component({
  components: {
    FontAwesomeIcon,
    ViamButton,
    ViamIcon,
  },
})
export default class RadioButtons extends Vue {
  @Prop() options!: [string];
  @Prop() defaultOption?: string;
  @Prop() disabledOptions?: [string];
  selected: string | undefined = "";
  mdiCheck = mdiCheck;
  mounted(): void {
    this.selected = this.defaultOption;
  }
  isDisabled(option: string): boolean {
    if (this.disabledOptions) {
      return !!this.disabledOptions.includes(option);
    } else {
      return false;
    }
  }
  @Watch("defaultOption")
  selectOption(option: string): void {
    this.selected = option;
    this.$emit("selectOption", option);
  }
}
</script>

<style scoped></style>
