import { storiesOf } from "@storybook/vue";

storiesOf("Tab", module).add("Default Tab", () => ({
  data() {
    return {
      streamName: "minirover",
      crumbs: ["Base", "4 Wheel"],
    };
  },
  template:
    "<Tabs>\n" +
    "  <Tab selected='true'>\n" +
    "    Keyboard\n" +
    "  </Tab>\n" +
    "  <Tab>\n" +
    "    Discrete\n" +
    "  </Tab>\n" +
    "  <Tab>\n" +
    "    Input\n" +
    "  </Tab>\n" +
    "</Tabs>\n" +
    "\n",
}));
