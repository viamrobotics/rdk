import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("WebGamepad", module).add("Default WebGamepad", () => ({
  data() {
    return {
      options: ["test1", "test2"]
    };
  },
  template:
    '<WebGamepad></WebGamepad>',
}));
