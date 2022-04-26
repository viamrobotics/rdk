import { storiesOf } from '@storybook/vue';

storiesOf('Base', module).add('Default Base', () => ({
  data() {
    return {
      streamName: 'minirover',
      crumbs: ['Base', '4 Wheel'],
    };
  },
  template:
    '<Base :streamName="streamName" :baseName="streamName" :crumbs="crumbs"></Base>',
}));
