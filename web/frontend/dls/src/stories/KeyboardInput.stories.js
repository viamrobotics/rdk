import { storiesOf } from "@storybook/vue";

storiesOf("KeyboardInput", module).add("Default KeyboardInput", () => ({
  data() {
    return {};
  },
  template: "<div><keyboard-input></keyboard-input></div>",
}));
