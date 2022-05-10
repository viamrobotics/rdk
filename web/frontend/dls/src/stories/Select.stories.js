import { storiesOf } from "@storybook/vue";

storiesOf("ViamSelect", module).add("Default ViamSelect", () => ({
  data() {
    return {
      value: 1,
      options: [
        { label: "no camera", value: 1 },
        { label: "camera", value: 2 },
      ],
    };
  },
  template:
    '<div><ViamSelect v-model="value" :options="options" value-key="value"></ViamSelect></div>',
}));
