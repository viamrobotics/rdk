import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("ViamInput", module).add("Default ViamInput", () => ({
  data() {
    return {
      streamNames: ["test1", "test2"]
    };
  },
  template:
    '<div><ViamInput color="primary" group="False" variant="primary">Primary</ViamButton></div>',
}));
