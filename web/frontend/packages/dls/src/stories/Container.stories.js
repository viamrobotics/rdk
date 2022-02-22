import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("Container", module).add("Default Container", () => ({
  data() {
    return {
      streamNames: ["test1", "test2"]
    };
  },
  template:
    '<Container>Content</Container>',
}));
