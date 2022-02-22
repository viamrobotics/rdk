import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("Camera", module).add("Default Camera", () => ({
  data() {
    return {
      streamNames: ["test1", "test2"]
    };
  },
  template:
    '<Camera :streamNames="streamNames"></Camera>',
}));
