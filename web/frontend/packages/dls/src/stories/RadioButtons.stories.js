import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("RadioButtons", module).add("Default RadioButtons", () => ({
  data() {
    return {
      options: ["test1", "test2"]
    };
  },
  template:
    '<RadioButtons :options="options"></RadioButtons>',
}));
