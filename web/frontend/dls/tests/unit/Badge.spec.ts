import { enableAutoDestroy, shallowMount } from '@vue/test-utils';
import Badge from '@/components/Badge.vue';

describe('ViamBadge', () => {
  enableAutoDestroy(afterEach);

  it('has html structure', async () => {
    const wrapper = shallowMount(Badge);

    expect(wrapper.element.tagName).toBe('SPAN');
    expect(wrapper.classes('inline-block')).toBe(true);
    expect(wrapper.classes('leading-tight')).toBe(true);
  });

  it('renders content properly', async () => {
    const wrapper = shallowMount(Badge, {
      slots: {
        default: '<a>content</a>',
      },
    });

    expect(wrapper.find('a').exists()).toBe(true);
    expect(wrapper.text()).toBe('content');
  });

  it('accepts different colors prop', async () => {
    const wrapper = shallowMount(Badge, {
      propsData: {
        color: 'red',
      },
    });

    expect(wrapper.classes('bg-red-200')).toBe(true);
    expect(wrapper.classes('text-red-900')).toBe(true);
  });

  it('renders root element', async () => {
    const wrapper = shallowMount(Badge, {
      propsData: {
        tag: 'div',
      },
    });

    expect(wrapper.element.tagName).toBe('DIV');
  });

  it('should emit events', async () => {
    let called = 0;
    let event = null;
    const wrapper = shallowMount(Badge, {
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
