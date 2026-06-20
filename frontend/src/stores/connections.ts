import { createSignal } from "solid-js";
import { ConfigService, type Connection, onEvent } from "~/lib/api";

const [connections, setConnections] = createSignal<Connection[]>([]);
export { connections };

/** Load all saved connections from the backend. */
export async function loadConnections() {
  try {
    const list = await ConfigService.List();
    setConnections(list ?? []);
  } catch (e) {
    console.error("loadConnections", e);
  }
}

/** Create or update a connection. */
export async function saveConnection(c: Connection): Promise<Connection> {
  const saved = await ConfigService.Save(c);
  await loadConnections();
  return saved;
}

/** Delete a connection by id. */
export async function deleteConnection(id: string) {
  await ConfigService.Delete(id);
  await loadConnections();
}

export function getConnection(id: string): Connection | undefined {
  return connections().find((c) => c.id === id);
}

// Connections are shared across windows: reload when any window changes them.
onEvent("configs:changed", () => loadConnections());
