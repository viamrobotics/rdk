import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("viamSwitch", module).add("Default viamSwitch", () => ({
  data() {
    return {
      option: false
    };
  },
  template:
    '<viamSwitch name="Test" id="test" :option=option></viamSwitch>',
}));
