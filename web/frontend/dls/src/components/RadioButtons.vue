<template>
  <div>
    <div class="inline-flex">
      <div
        class="relative inline-flex"
        :data-cy="'button-wrapper-' + option"
        :key="option"
        v-for="(option, i) in options"
      >
        <ViamButton
          group
          variant="primary"
          v-bind:key="option"
          :class="{ 'border-r-0': i < options.length - 1, 'border-l-0': i > 0 }"
          class="py-1 px-4 radio-button"
          :data-cy="'button-' + option"
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
        <div
          v-show="
            i < options.length - 1 &&
            (i - getSelectedIndex > 0 || Math.abs(getSelectedIndex - i) > 1)
          "
          class="absolute w-px top-2 bottom-2 right-0 bg-gray-300"
        ></div>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { Component, Prop, Vue, Watch } from 'vue-property-decorator';
import 'vue-class-component/hooks';
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome';
import { mdiCheck } from '@mdi/js';
import ViamButton from './Button.vue';
import ViamIcon from './ViamIcon.vue';
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
  selected: string | undefined = '';

  mdiCheck = mdiCheck;
  mounted(): void {
    this.selected = this.defaultOption;
  }

  get getSelectedIndex(): number {
    let foundIndex = -1;
    for (let i = 0; i < this.options.length; i++) {
      if (this.options[i] === this.selected) foundIndex = i;
    }
    return foundIndex;
  }

  isDisabled(option: string): boolean {
    if (this.disabledOptions) {
      return !!this.disabledOptions.includes(option);
    } else {
      return false;
    }
  }
  @Watch('defaultOption')
  selectOption(option: string): void {
    this.selected = option;
    this.$emit('selectOption', option);
  }
}
</script>
