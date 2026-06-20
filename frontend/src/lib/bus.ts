// A minimal in-window event bus for cross-component UI actions (menu shortcuts,
// "new tab", "refresh", etc.) that don't belong in a store.
type Handler = (payload?: any) => void;

const handlers: Record<string, Set<Handler>> = {};

export function on(event: string, h: Handler): () => void {
  (handlers[event] ??= new Set()).add(h);
  return () => handlers[event]?.delete(h);
}

export function emit(event: string, payload?: any) {
  handlers[event]?.forEach((h) => h(payload));
}

/** Play a short notification beep via the Web Audio API (no asset needed). */
export function playBeep(isError = false) {
  try {
    const ctx = new (window.AudioContext || (window as any).webkitAudioContext)();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.connect(gain);
    gain.connect(ctx.destination);
    osc.type = "sine";
    osc.frequency.value = isError ? 320 : 660;
    gain.gain.setValueAtTime(0.0001, ctx.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.15, ctx.currentTime + 0.01);
    gain.gain.exponentialRampToValueAtTime(0.0001, ctx.currentTime + 0.3);
    osc.start();
    osc.stop(ctx.currentTime + 0.32);
    osc.onended = () => ctx.close();
  } catch {
    /* ignore audio failures */
  }
}
