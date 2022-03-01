import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("Breadcrumbs", module).add("Default Breadcrumbs", () => ({
  data() {
    return {
      crumbs: ["test1", "test2"]
    };
  },
  template:
    '<div><Breadcrumbs :crumbs="crumbs"></Breadcrumbs></div>',
}));
