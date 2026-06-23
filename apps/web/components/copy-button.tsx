"use client";

import * as React from "react";
import { Check, Copy } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

interface CopyButtonProps {
  value: string;
  label?: string;
  size?: "sm" | "icon";
  className?: string;
  variant?: React.ComponentProps<typeof Button>["variant"];
}

/**
 * Copy-to-clipboard button composed from the shadcn Button + Tooltip primitives.
 * No bespoke CSS — only utility classes and theme tokens.
 */
export function CopyButton({
  value,
  label = "Copy",
  size = "icon",
  className,
  variant = "ghost",
}: CopyButtonProps) {
  const [copied, setCopied] = React.useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      toast.success("Copied to clipboard");
      setTimeout(() => setCopied(false), 1500);
    } catch {
      toast.error("Could not copy to clipboard");
    }
  }

  const icon = copied ? (
    <Check className="size-4" />
  ) : (
    <Copy className="size-4" />
  );

  if (size === "sm") {
    return (
      <Button
        type="button"
        variant={variant}
        size="sm"
        onClick={copy}
        className={className}
      >
        {icon}
        {label}
      </Button>
    );
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant={variant}
          size="icon"
          onClick={copy}
          className={cn("size-8", className)}
          aria-label={label}
        >
          {icon}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}
