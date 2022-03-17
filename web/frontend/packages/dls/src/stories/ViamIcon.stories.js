import { storiesOf } from "@storybook/vue";

storiesOf("ViamIcon", module).add("Default ViamIcon", () => ({
  data() {
    return {
      name: "rotate-cw",
    };
  },
  template: '<div><ViamIcon  name="rotate-cw">Test Content</ViamIcon></div>',
}));
