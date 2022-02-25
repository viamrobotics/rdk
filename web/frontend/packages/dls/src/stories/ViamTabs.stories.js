import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("ViamTabs", module).add("Default ViamTabs", () => ({
  data() {
    return {
      streamNames: ["test1", "test2"]
    };
  },
  template:
    '<div><ViamTabs><ViamTabsItem>Tab 1</ViamTabsItem><ViamTabsItem selected>Tab 2</ViamTabsItem></ViamTabs></div>',
}));
