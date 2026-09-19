import { describe, it, expect } from "vitest";
import { dimensions, newPiece, newFabric } from "./types";
describe("unknown stock dimensions", () => {
  it("keeps unknown different from zero", () => {
    expect(dimensions(newPiece())).toBe("待测 × 待测 cm");
    expect(newFabric().pieces).toHaveLength(1);
  });
  it("distinguishes irregular pieces and counts", () => {
    expect(
      dimensions({
        width: "150",
        length: "80",
        unit: "cm",
        count: 2,
        irregular: true,
        note: "",
      }),
    ).toBe("150 × 80 cm · 2 片 · 不规则");
  });
});
