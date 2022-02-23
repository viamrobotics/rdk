import { storiesOf } from "@storybook/vue";
import { withDesign } from 'storybook-addon-designs'

storiesOf("Collapse", module).add("Default Collapse", () => ({
  data() {
    return {
      streamNames: ["test1", "test2"]
    };
  },
  template:
    '<Collapse>Test<template v-slot:content><div class="content">Test content</div></template></Collapse>',
}));
