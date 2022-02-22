import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("ToggleButton", module).add("Default ToggleButton", () => ({
  data() {
    return {
      option: true
    };
  },
  template:
    '<ToggleButton></ToggleButton>',
}));
