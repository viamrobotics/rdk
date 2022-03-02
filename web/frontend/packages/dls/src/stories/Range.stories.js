import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("Range", module).add("Default Range", () => ({
  data() {
    return {
      percentage: 50,
      name: "Test Range:"
    };
  },
  template:
    '<div><Range :percentage=percentage :name="name"></Range></div>',
}));
