import type { ApiResponse } from "@/types";

async function apiRequest<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
    ...options,
  });

  if (res.status === 401) {
    const next = encodeURIComponent(window.location.pathname + window.location.search);
    window.location.href = `/login?next=${next}`;
    throw new Error("Unauthorized");
  }

  if (!res.ok) {
    let msg = `HTTP ${res.status}`;
    try {
      const body = (await res.json()) as ApiResponse<unknown>;
      if (body.error) msg = body.error;
    } catch {
      // ignore parse errors
    }
    throw new Error(msg);
  }

  const body = (await res.json()) as ApiResponse<T>;

  if (!body.success) {
    throw new Error(body.error || "Unknown error");
  }

  return body.data;
}

export function get<T>(url: string): Promise<T> {
  return apiRequest<T>(url, { method: "GET" });
}

export function post<T>(url: string, data?: unknown): Promise<T> {
  return apiRequest<T>(url, {
    method: "POST",
    body: data !== undefined ? JSON.stringify(data) : undefined,
  });
}

export function put<T>(url: string, data?: unknown): Promise<T> {
  return apiRequest<T>(url, {
    method: "PUT",
    body: data !== undefined ? JSON.stringify(data) : undefined,
  });
}

export function patch<T>(url: string, data?: unknown): Promise<T> {
  return apiRequest<T>(url, {
    method: "PATCH",
    body: data !== undefined ? JSON.stringify(data) : undefined,
  });
}

export function del<T>(url: string): Promise<T> {
  return apiRequest<T>(url, { method: "DELETE" });
}

export async function downloadFile(url: string, filename: string): Promise<void> {
  const res = await fetch(url, { credentials: "include" });

  if (res.status === 401) {
    const next = encodeURIComponent(window.location.pathname + window.location.search);
    window.location.href = `/login?next=${next}`;
    throw new Error("Unauthorized");
  }

  if (!res.ok) {
    throw new Error(`Download failed: HTTP ${res.status}`);
  }

  const blob = await res.blob();
  const objectUrl = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = objectUrl;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(objectUrl);
}
