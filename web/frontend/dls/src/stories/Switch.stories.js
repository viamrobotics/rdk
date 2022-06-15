import { storiesOf } from '@storybook/vue';

storiesOf('ViamSwitch', module).add('Default viamSwitch', () => ({
  data() {
    return {
      option: false,
    };
  },
  template:
    '<ViamSwitch name="Test" size="sm" id="test" :option=option @change="val => {option = val}"></ViamSwitch>',
}));
