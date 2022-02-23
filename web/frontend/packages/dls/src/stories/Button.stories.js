import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("Button", module).add("Default Button", () => ({
  data() {
    return {
      streamNames: ["test1", "test2"]
    };
  },
  template:
    '<ViamButton>Test</ViamButton>',
}));
