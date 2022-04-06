import { storiesOf } from "@storybook/vue";

storiesOf("Breadcrumbs", module).add("Default Breadcrumbs", () => ({
  data() {
    return {
      crumbs: ["test1", "test2"],
    };
  },
  template: '<div><Breadcrumbs :crumbs="crumbs"></Breadcrumbs></div>',
}));
