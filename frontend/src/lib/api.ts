async function readError(response: Response) {
  const text = await response.text();
  if (!text) {
    throw new Error(`request failed: ${response.status}`);
  }
  throw new Error(`request failed: ${response.status} ${text}`);
}

export async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(path, { credentials: "include" });
  if (!response.ok) {
    await readError(response);
  }
  return response.json() as Promise<T>;
}

export async function postJSON<T>(path: string, body?: unknown): Promise<T> {
  const response = await fetch(path, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    await readError(response);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  const text = await response.text();
  if (!text) {
    return undefined as T;
  }
  return JSON.parse(text) as T;
}

export async function getText(path: string): Promise<string> {
  const response = await fetch(path, { credentials: "include" });
  if (!response.ok) {
    await readError(response);
  }
  return response.text();
}

export async function putJSON(path: string, body: unknown): Promise<void> {
  const response = await fetch(path, {
    method: "PUT",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    await readError(response);
  }
}

export async function putText(path: string, body: string, contentType = "application/yaml; charset=utf-8"): Promise<void> {
  const response = await fetch(path, {
    method: "PUT",
    credentials: "include",
    headers: {
      "Content-Type": contentType,
    },
    body,
  });
  if (!response.ok) {
    await readError(response);
  }
}
