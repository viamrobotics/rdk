import { storiesOf } from '@storybook/vue';

storiesOf('Tab', module).add('Default Tab', () => ({
  data() {
    return {
      streamName: 'minirover',
      crumbs: ['Base', '4 Wheel'],
      selectedItem: 'keyboard',
    };
  },
  template: `
          <Tabs>
          <Tab :selected='selectedItem === "keyboard"' @select='selectedItem = "keyboard"'>
            Keyboard
          </Tab>
          <Tab :selected='selectedItem === "discrete"' @select='selectedItem = "discrete"'>
            Discrete
          </Tab>
          <Tab :selected='selectedItem === "input"' @select='selectedItem = "input"'>
            Input
          </Tab>
          </Tabs>
        `,
}));
