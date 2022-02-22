import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("Accordion", module).add("Default Accordion", () => ({
  data() {
    return {
      streamNames: ["test1", "test2"]
    };
  },
  template:
    '<Accordion></Accordion>',
}));
