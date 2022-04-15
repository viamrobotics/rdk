import { storiesOf } from "@storybook/vue";

storiesOf("Camera", module).add("Default Camera", () => ({
  data() {
    return {
      streamName: "Camera1",
      crumbs: ["Camera", "Intel"],
      segmentAlgo: ["Camera", "Intel"],
    };
  },
  template:
    '<Camera :streamName="streamName" :segmenterNames="segmentAlgo" :segmentAlgo="segmentAlgo" :crumbs="crumbs"></Camera>',
}));
