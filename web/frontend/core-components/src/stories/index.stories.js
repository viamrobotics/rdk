import MotorDetail from "../components/MotorDetail.vue";

export default {
  title: "MotorDetail",
};

const Template = (args) => ({
  // Components used in your story `template` are defined in the `components` object
  components: { MotorDetail },
  // The story's `args` need to be mapped into the template through the `setup()` method
  setup() {
    return { args };
  },
  // And then the `args` are bound to your component with `v-bind="args"`
  template: '<div id="app">\n' +
      '    <MotorDetail\n' +
      '      motorName="MOTOR NAME"\n' +
      '      :motorStatus="{\n' +
      '        on: false,\n' +
      '        positionSupported: true,\n' +
      '        position: 0,\n' +
      '        pidConfig: {\n' +
      '          fieldsMap: [\n' +
      '            [\'Kd\', { numberValue: 0 }],\n' +
      '            [\'Kp\', { numberValue: 0.34 }],\n' +
      '            [\'Ki\', { numberValue: 0.77 }],\n' +
      '          ],\n' +
      '        },\n' +
      '      }"\n' +
      '    />\n' +
      '  </div>',
});

export const Primary = Template.bind({});
Primary.args = {};