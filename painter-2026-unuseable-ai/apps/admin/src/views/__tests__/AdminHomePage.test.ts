import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import AdminHomePage from "../AdminHomePage.vue";

describe("AdminHomePage", () => {
  it("renders heading", () => {
    const wrapper = mount(AdminHomePage);
    expect(wrapper.text()).toContain("管理后台总览");
  });
});
