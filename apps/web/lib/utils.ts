import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

/** cn merges class names with Tailwind-aware conflict resolution (shadcn standard). */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
