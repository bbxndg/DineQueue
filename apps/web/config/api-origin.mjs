export function apiOrigin(value = "http://127.0.0.1:8080") {
  try {
    const url = new URL(value);
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password ||
        url.pathname !== '/' || url.search || url.hash) {
      throw new Error();
    }
    return url.origin;
  } catch {
    throw new Error('API_INTERNAL_URL must be an HTTP(S) origin without credentials or a path');
  }
}
