import { storiesOf } from '@storybook/vue';

storiesOf('RadioButtons', module).add('Default RadioButtons', () => ({
  data() {
    return {
      streamNames: ['test1', 'test2'],
    };
  },
  template: `
    <RadioButtons
      :options="['Straight', 'Arc', 'Spin']"
      :disabledOptions="[]"
    />
    `,
}));
