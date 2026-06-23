"use client";

import * as React from "react";
import { ThemeProvider as NextThemesProvider } from "next-themes";

/**
 * ThemeProvider is the exact shadcn/ui dark-mode recipe (next-themes). Dark mode
 * is built-in; no custom CSS is written for theming — colors come only from the
 * shadcn theme tokens in app/globals.css.
 */
export function ThemeProvider({
  children,
  ...props
}: React.ComponentProps<typeof NextThemesProvider>) {
  return <NextThemesProvider {...props}>{children}</NextThemesProvider>;
}
