import { afterEach, describe, expect, it, vi } from "vitest";
import { redirectTo } from "../lib/navigation";

describe("redirectTo", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("redirects in browser environments", () => {
    const replace = vi.fn();

    vi.stubGlobal("window", {
      location: {
        replace,
      },
    });

    redirectTo("/login");

    expect(replace).toHaveBeenCalledTimes(1);
    expect(replace).toHaveBeenCalledWith("/login");
  });

  it("is safe during server-side execution", () => {
    vi.stubGlobal("window", undefined);

    expect(() => redirectTo("/login")).not.toThrow();
  });
});
