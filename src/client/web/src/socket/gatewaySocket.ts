const OPEN_WEBSOCKET = 1;

export type SocketLike = {
  readyState: number;
  url?: string | null;
};

export function isGatewaySocketURL(url: string | null | undefined): boolean {
  if (!url) return false;
  try {
    const parsed = new URL(url, "http://127.0.0.1");
    return parsed.pathname === "/ws" && parsed.searchParams.get("ticket") !== null;
  } catch {
    return false;
  }
}

export function isOpenGatewaySocket(socket: SocketLike): boolean {
  return socket.readyState === OPEN_WEBSOCKET && isGatewaySocketURL(socket.url);
}
