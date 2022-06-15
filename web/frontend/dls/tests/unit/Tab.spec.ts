import { enableAutoDestroy, shallowMount } from '@vue/test-utils';
import Tab from '@/components/Tab.vue';

describe('Tab', () => {
  enableAutoDestroy(afterEach);

  it('has html structure', async () => {
    const wrapper = shallowMount(Tab);

    expect(wrapper.element.tagName).toBe('BUTTON');
    expect(wrapper.classes('cursor-pointer')).toBe(true);
    expect(wrapper.attributes('aria-disabled')).not.toBeDefined();
    expect(wrapper.attributes('aria-selected')).toBe('false');
  });

  it('renders content inside the slot', async () => {
    const wrapper = shallowMount(Tab, {
      slots: {
        default: '<span>foobar</span>',
      },
    });

    expect(wrapper.find('span').exists()).toBe(true);
    expect(wrapper.text()).toBe('foobar');
  });

  it('renders custom root element', async () => {
    const wrapper = shallowMount(Tab, {
      propsData: {
        tag: 'section',
      },
    });

    expect(wrapper.element.tagName).toBe('SECTION');
  });

  it('accepts selected property', () => {
    const wrapper = shallowMount(Tab, {
      propsData: {
        selected: true,
      },
    });

    expect(wrapper.attributes('aria-selected')).toBe('true');
  });

  it('has attribute disabled when disabled set', () => {
    const wrapper = shallowMount(Tab, {
      propsData: {
        disabled: true,
      },
    });

    expect(wrapper.attributes('aria-disabled')).toBe('true');
    expect(wrapper.classes('cursor-pointer')).toBe(false);
    expect(wrapper.classes('cursor-not-allowed')).toBe(true);
  });

  it('should emit events', async () => {
    let called = 0;
    let event = null;
    const wrapper = shallowMount(Tab, {
      listeners: {
        blur: (e: Event) => {
          event = e;
          called += 1;
        },
        click: (e: Event) => {
          event = e;
          called += 1;
        },
        focus: (e: Event) => {
          event = e;
          called += 1;
        },
      },
    });

    expect(called).toBe(0);
    expect(event).toEqual(null);

    await wrapper.trigger('click');
    expect(called).toBe(1);
    expect(event).toBeInstanceOf(MouseEvent);

    wrapper.element.dispatchEvent(new Event('focus'));
    expect(called).toBe(2);
    expect(event).toBeInstanceOf(Event);

    wrapper.element.dispatchEvent(new Event('blur'));
    expect(called).toBe(3);
    expect(event).toBeInstanceOf(Event);
  });
});
