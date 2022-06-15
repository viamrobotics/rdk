import { storiesOf } from '@storybook/vue';

storiesOf('ViamBadge', module).add('Default ViamBadge', () => ({
  data() {
    return {
      streamNames: ['test1', 'test2'],
    };
  },
  template:
    '<div><ViamBadge color="green">Green</ViamBadge><ViamBadge color="orange">Orange</ViamBadge><ViamBadge color="red">Red</ViamBadge></div>',
}));
