import { storiesOf } from '@storybook/vue';

storiesOf('Camera', module).add('Default Camera', () => ({
  data() {
    return {
      first: {
        streamName: 'Camera1',
        crumbs: ['Camera', 'Intel'],
        segmentAlgo: ['Camera', 'Intel'],
      },
      second: {
        streamName: 'Camera2',
        crumbs: ['Camera2', 'Intel2'],
        segmentAlgo: ['Camera3', 'Intel3'],
      },
    };
  },
  template: `
    <div>
    <Camera :streamName="first.streamName" :segmenterNames="first.segmentAlgo" :segmentAlgo="first.segmentAlgo" :crumbs="first.crumbs"></Camera>
    <Camera :streamName="second.streamName" :segmenterNames="second.segmentAlgo" :segmentAlgo="second.segmentAlgo" :crumbs="second.crumbs"></Camera>
    </div>
  `,
}));
