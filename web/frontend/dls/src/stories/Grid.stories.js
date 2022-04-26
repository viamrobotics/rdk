import { storiesOf } from '@storybook/vue';

storiesOf('Grid', module).add('Default Grid', () => ({
  data() {
    return {
      streamNames: ['test1', 'test2'],
    };
  },
  template:
    '<Grid cols="3"><div>Test 1</div><div>Test 2</div><div>Test 3</div></Grid>',
}));
