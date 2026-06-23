"use client";

import { CopyButton } from "@/components/copy-button";
import { ScrollArea, ScrollBar } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";

interface CodeBlockProps {
  code: string;
  className?: string;
}

/**
 * Code block composed from primitives only: a muted-background container with a
 * mono font, a horizontal ScrollArea, and a copy Button. No <style>, no bespoke CSS.
 */
export function CodeBlock({ code, className }: CodeBlockProps) {
  return (
    <div className={cn("bg-muted relative rounded-md border", className)}>
      <div className="absolute right-2 top-2 z-10">
        <CopyButton value={code} variant="secondary" />
      </div>
      <ScrollArea className="w-full">
        <pre className="px-4 py-3 pr-12 font-mono text-sm leading-relaxed">
          <code>{code}</code>
        </pre>
        <ScrollBar orientation="horizontal" />
      </ScrollArea>
    </div>
  );
}
