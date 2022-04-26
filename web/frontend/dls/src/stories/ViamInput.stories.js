import { storiesOf } from '@storybook/vue';

storiesOf('ViamInput', module).add('Default ViamInput', () => ({
  data() {
    return {
      streamNames: ['test1', 'test2'],
    };
  },
  template:
    '<div><ViamInput color="primary" group="False" variant="primary">Primary</ViamInput></div>',
}));
