import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import TropicalAtmosphere from "../components/TropicalAtmosphere";

describe("TropicalAtmosphere", () => {
  it("renders deterministic decorative leaves", () => {
    const { container } = render(<TropicalAtmosphere />);
    const shell = container.querySelector(".botanical-atmosphere");
    expect(shell).not.toBeNull();
    expect(shell?.getAttribute("aria-hidden")).toBe("true");
    expect(container.querySelectorAll("svg.floating-leaf")).toHaveLength(11);
    expect(container.querySelectorAll("path.leaf-vein")).toHaveLength(11);
  });
});
