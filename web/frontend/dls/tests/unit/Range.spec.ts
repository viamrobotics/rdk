import { enableAutoDestroy, mount } from "@vue/test-utils";
import Range from "@/components/Range.vue";

describe("Range", () => {
  enableAutoDestroy(afterEach);

  it("has html structure", async () => {
    const wrapper = mount({
      data() {
        return { value: 12 };
      },
      template: '<div> <Range v-model="value"></Range></div>',
      components: { Range },
    });
    const input = wrapper.find("input").element as HTMLInputElement;
    expect(wrapper.element.tagName).toBe("DIV");
    expect(input.value).toBe("12");
  });
});
