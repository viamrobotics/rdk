import { enableAutoDestroy, mount, shallowMount } from "@vue/test-utils";
import ViamInfoButton from "@/components/ViamInfoButton.vue";
import { mdiInformationOutline } from "@mdi/js";

describe("ViamInfoButton", () => {
  enableAutoDestroy(afterEach);
  beforeEach(() => {
    document.createRange = () => ({
      setStart: () => ({}),
      setEnd: () => ({}),
      // eslint-disable-next-line
      //@ts-ignore
      commonAncestorContainer: {
        nodeName: "BODY",
        ownerDocument: document,
      },
    });
  });

  it("has html structure renders info rows", async () => {
    const wrapper = mount({
      data() {
        return {
          infoRows: [
            "When turned on, point cloud will be recalculated",
            "2. Another line",
            "three is not my limit",
          ],
          mdi: mdiInformationOutline,
        };
      },
      template: `<div> <viam-info-button :infoRows="infoRows" :iconPath="mdi"></viam-info-button> </div>`,
      components: { ViamInfoButton },
    });
    expect(wrapper.element.tagName).toBe("DIV");
    const button = wrapper.find("[data-cy=viam-info-button-container]");
    button.trigger("click");
    const rowItems = wrapper.findAll("[data-cy=viam-info-row-item]");
    expect(rowItems.length).toBe(3);
    const items = rowItems.wrappers.map((el) =>
      (el.element as HTMLElement).innerHTML.trim()
    );
    expect(items[0]).toBe("When turned on, point cloud will be recalculated");
    expect(items[1]).toBe("2. Another line");
    expect(items[2]).toBe("three is not my limit");
  });
});
