/**
 * swipe.test.ts — unit tests for swipe util (FR-DRAWER-005 / m1-swipe-util spec).
 */

import { describe, expect, it } from "vitest";
import { isLeftToRightSwipe, pointFromTouch, swipeDelta } from "./swipe";

describe("swipeDelta", () => {
  it("returns correct dx/dy for rightward move", () => {
    expect(swipeDelta({ x: 0, y: 0 }, { x: 80, y: 20 })).toEqual({ dx: 80, dy: 20 });
  });

  it("returns negative dx for leftward move", () => {
    expect(swipeDelta({ x: 100, y: 0 }, { x: 20, y: 5 })).toEqual({ dx: -80, dy: 5 });
  });
});

describe("pointFromTouch", () => {
  it("extracts x=clientX and y=clientY from a Touch", () => {
    const t = { clientX: 42, clientY: 99 } as Touch;
    expect(pointFromTouch(t)).toEqual({ x: 42, y: 99 });
  });
});

describe("isLeftToRightSwipe", () => {
  it("threshold exactly met (dx=50, dy=29) → true", () => {
    expect(isLeftToRightSwipe({ x: 0, y: 0 }, { x: 50, y: 29 })).toBe(true);
  });

  it("dy at boundary (dx=50, dy=30) → false (|dy| < 30 is strict)", () => {
    expect(isLeftToRightSwipe({ x: 0, y: 0 }, { x: 50, y: 30 })).toBe(false);
  });

  it("dx just below threshold (dx=49, dy=10) → false", () => {
    expect(isLeftToRightSwipe({ x: 0, y: 0 }, { x: 49, y: 10 })).toBe(false);
  });

  it("right→left direction (dx=-100, dy=5) → false", () => {
    expect(isLeftToRightSwipe({ x: 100, y: 0 }, { x: 0, y: 5 })).toBe(false);
  });

  it("clear left→right swipe (dx=80, dy=20) → true", () => {
    expect(isLeftToRightSwipe({ x: 0, y: 0 }, { x: 80, y: 20 })).toBe(true);
  });

  it("custom minDx=100: dx=80 → false", () => {
    expect(isLeftToRightSwipe({ x: 0, y: 0 }, { x: 80, y: 10 }, { minDx: 100 })).toBe(false);
  });

  it("custom minDx=100: dx=100 → true", () => {
    expect(isLeftToRightSwipe({ x: 0, y: 0 }, { x: 100, y: 10 }, { minDx: 100 })).toBe(true);
  });

  it("negative dy (upward drift) absolute value check: dx=80, dy=-25 → true", () => {
    expect(isLeftToRightSwipe({ x: 0, y: 100 }, { x: 80, y: 75 })).toBe(true);
  });

  it("negative dy (upward drift) over threshold: dx=80, dy=-30 → false", () => {
    expect(isLeftToRightSwipe({ x: 0, y: 100 }, { x: 80, y: 70 })).toBe(false);
  });
});
