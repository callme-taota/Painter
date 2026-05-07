import { describe, expect, it } from "vitest";
import { usePagination } from "../usePagination";

describe("usePagination", () => {
  it("updates cursor", () => {
    const { cursor, updateCursor } = usePagination();
    updateCursor("10");
    expect(cursor.value).toBe("10");
  });
});
