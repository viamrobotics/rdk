import { enableAutoDestroy, shallowMount } from '@vue/test-utils';
import Collapse from '@/components/Collapse.vue';

describe('Collapse', () => {
  enableAutoDestroy(afterEach);

  it('has html structure', async () => {
    const wrapper = shallowMount(Collapse);

    expect(wrapper.element.tagName).toBe('DIV');
    expect(wrapper.attributes('aria-expanded')).toBe('false');
    expect(wrapper.attributes('aria-disabled')).not.toBeDefined();
  });

  it('Render content inside a slot', async () => {
    const wrapper = shallowMount(Collapse, {
      slots: {
        default: '<span>content</span>',
      },
    });

    expect(wrapper.find('span').exists()).toBe(true);
    expect(wrapper.text()).toBe('content');
  });

  it('renders customized root element', async () => {
    const wrapper = shallowMount(Collapse, {
      propsData: {
        tag: 'section',
      },
    });

    expect(wrapper.element.tagName).toBe('SECTION');
  });

  it('with disabled property', async () => {
    const wrapper = shallowMount(Collapse, {
      propsData: {
        disabled: true,
      },
      slots: {
        default: '<span>disabled</span>',
      },
    });

    expect(wrapper.attributes('aria-disabled')).toBeDefined();
    expect(wrapper.attributes('aria-disabled')).toBe('true');
    await wrapper.find('span').trigger('click');
  });
});
