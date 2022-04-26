import { shallowMount } from '@vue/test-utils';
import RadioButtons from '@/components/RadioButtons.vue';

describe('RadioButtons.vue', () => {
  it('renders props.options when passed', () => {
    const wrapper = shallowMount(RadioButtons, ['Continuous', 'Discrete']);
    expect(wrapper.get('div')).toBeTruthy();
  });
});
