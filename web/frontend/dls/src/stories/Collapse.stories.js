import { storiesOf } from "@storybook/vue";

storiesOf("Collapse", module).add("Default Collapse", () => ({
  data() {
    return {
      streamNames: ["test1", "test2"],
    };
  },
  template:
    '<Collapse>Test<template v-slot:content><div class="content p-2">Test content</div></template></Collapse>',
}));
