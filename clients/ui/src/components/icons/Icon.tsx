import type { JSX, SVGProps } from "react";
import "../../css/icon.css";
import { ICON_PATHS, type IconName } from "./paths";

export type { IconName };

export interface IconProps extends Omit<SVGProps<SVGSVGElement>, "children"> {
  name: IconName;
  /** Pixel size (width and height). Default 16. */
  size?: number;
}

/**
 * Static lucide SVG icon (ADR-20260714-behavior-lib-and-icons).
 * Stroke width fixed at 1.5; color inherits via currentColor.
 */
export function Icon({ name, size = 16, className, ...rest }: IconProps): JSX.Element {
  const cls = ["icon", `icon--${name}`, className].filter(Boolean).join(" ");
  return (
    <svg
      {...rest}
      className={cls}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {ICON_PATHS[name]}
    </svg>
  );
}
