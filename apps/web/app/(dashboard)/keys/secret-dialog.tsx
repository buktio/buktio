"use client";

import * as React from "react";
import { TriangleAlert } from "lucide-react";

import type { CreatedAccessKey } from "@/lib/api";
import { CopyButton } from "@/components/copy-button";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

/**
 * Shows the freshly-created access key with its one-time secret. The user must
 * confirm they saved it before the dialog can be closed.
 */
export function SecretDialog({
  created,
  onClose,
}: {
  created: CreatedAccessKey | null;
  onClose: () => void;
}) {
  const [acknowledged, setAcknowledged] = React.useState(false);

  // Reset the gate whenever a new key is shown.
  React.useEffect(() => {
    if (created) setAcknowledged(false);
  }, [created]);

  return (
    <Dialog
      open={!!created}
      onOpenChange={(o) => {
        // Prevent closing (escape/overlay) until the user acknowledges.
        if (!o && acknowledged) onClose();
      }}
    >
      <DialogContent
        className="sm:max-w-lg"
        onEscapeKeyDown={(e) => {
          if (!acknowledged) e.preventDefault();
        }}
        onInteractOutside={(e) => {
          if (!acknowledged) e.preventDefault();
        }}
        showCloseButton={false}
      >
        <DialogHeader>
          <DialogTitle>Access key created</DialogTitle>
          <DialogDescription>
            Copy your secret access key now. For security, it will never be shown again.
          </DialogDescription>
        </DialogHeader>

        {created && (
          <div className="flex flex-col gap-4 py-2">
            <div className="grid gap-2">
              <Label htmlFor="secret-access-key-id">Access key ID</Label>
              <div className="flex gap-2">
                <Input
                  id="secret-access-key-id"
                  readOnly
                  value={created.access_key_id}
                  className="font-mono text-xs"
                />
                <CopyButton value={created.access_key_id} variant="outline" label="Copy ID" />
              </div>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="secret-access-key">Secret access key</Label>
              <div className="flex gap-2">
                <Input
                  id="secret-access-key"
                  readOnly
                  value={created.secret_access_key}
                  className="font-mono text-xs"
                />
                <CopyButton
                  value={created.secret_access_key}
                  variant="outline"
                  label="Copy secret"
                />
              </div>
            </div>

            <Alert variant="destructive">
              <TriangleAlert className="size-4" />
              <AlertTitle>Save this secret now</AlertTitle>
              <AlertDescription>
                This is the only time the secret access key is displayed. If you lose it, you must
                delete this key and create a new one.
              </AlertDescription>
            </Alert>

            <label className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={acknowledged}
                onCheckedChange={(c) => setAcknowledged(c === true)}
              />
              I have securely saved my secret access key.
            </label>
          </div>
        )}

        <DialogFooter>
          <Button disabled={!acknowledged} onClick={onClose}>
            Done
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
