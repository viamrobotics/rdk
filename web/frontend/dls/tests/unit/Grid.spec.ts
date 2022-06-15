import { enableAutoDestroy, shallowMount } from '@vue/test-utils';
import Grid from '@/components/Grid.vue';

describe('Grid', () => {
  enableAutoDestroy(afterEach);

  it('has html structure', async () => {
    const wrapper = shallowMount(Grid);

    expect(wrapper.element.tagName).toBe('DIV');
    expect(wrapper.classes('grid')).toBe(true);
  });

  it('render content inside a slot', async () => {
    const wrapper = shallowMount(Grid, {
      slots: {
        default: '<span>foobar</span>',
      },
    });

    expect(wrapper.find('span').exists()).toBe(true);
    expect(wrapper.text()).toBe('foobar');
  });

  it('renders root element', async () => {
    const wrapper = shallowMount(Grid, {
      propsData: {
        tag: 'section',
      },
    });

    expect(wrapper.element.tagName).toBe('SECTION');
  });

  it('accepts cols and gap properties', async () => {
    const wrapper = shallowMount(Grid, {
      propsData: {
        cols: '2',
        gap: '4',
      },
    });

    expect(wrapper.classes('sm:grid-cols-2')).toBe(true);
    expect(wrapper.classes('sm:gap-4')).toBe(true);
  });
});
