import { createSignal, onCleanup } from "solid-js";
import { AppService, newOperationID } from "~/lib/api";

/** Thrown-free result of a cancellable run: `null` means the user aborted it. */
export type OpResult<T> = T | null;

/**
 * Drives one long-running backend call that the user can abandon.
 *
 * A recursive search or a prefix walk can take minutes on a large bucket, and
 * without a handle on it the only option is to wait out the server-side timeout.
 * Each run gets an id the backend registers a cancellable context under; calling
 * `cancel()` aborts it there rather than merely ignoring the reply here.
 */
export function createCancellableOp() {
  const [running, setRunning] = createSignal(false);
  let currentID: string | null = null;

  /**
   * Runs `fn` with a fresh operation id. Resolves to `null` when the run was
   * cancelled (by the user, or by a later run superseding it); any other failure
   * is rethrown for the caller to report.
   */
  async function run<T>(fn: (opID: string) => Promise<T>): Promise<OpResult<T>> {
    // A new run supersedes whatever is in flight.
    if (currentID) void AppService.CancelOperation(currentID);
    const id = newOperationID();
    currentID = id;
    setRunning(true);
    try {
      const result = await fn(id);
      // A superseded run must not overwrite the newer one's result.
      return currentID === id ? result : null;
    } catch (e) {
      if (currentID !== id || wasCancelled(e)) return null;
      throw e;
    } finally {
      if (currentID === id) {
        currentID = null;
        setRunning(false);
      }
    }
  }

  function cancel() {
    if (!currentID) return;
    void AppService.CancelOperation(currentID);
    currentID = null;
    setRunning(false);
  }

  onCleanup(cancel);

  return { running, run, cancel };
}

/** Recognise the backend's cancellation error, which is not worth surfacing. */
function wasCancelled(e: unknown): boolean {
  const message = String((e as { message?: string })?.message ?? e);
  return message.includes("context canceled") || message.includes("operation canceled");
}
