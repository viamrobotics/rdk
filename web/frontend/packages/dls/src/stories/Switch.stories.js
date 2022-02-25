import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("ViamSwitch", module).add("Default viamSwitch", () => ({
  data() {
    return {
      option: false
    };
  },
  template:
    '<ViamSwitch name="Test" id="test" :option=option @change="val => {option = val}"></ViamSwitch>',
}));