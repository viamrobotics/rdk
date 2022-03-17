import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs';

storiesOf("InputController", module).add("Default InputController", () => ({
  data() {
    return {
      status: {
        eventsList: [
          {
            event: "test",
            control: "test",
            value: 1,
          },
        ],
      },
    };
  },
  template:
    '<InputController controllerName="test" :controllerStatus="status"></InputController>',
}));
