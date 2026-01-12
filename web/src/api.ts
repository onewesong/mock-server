import type { Endpoint, Rule, PreviewRequest, PreviewResponse, ExportBundle } from "./types";

async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(url, {
    headers: {
      "Content-Type": "application/json",
      ...(options.headers || {})
    },
    ...options
  });
  if (!res.ok) {
    const payload = await res.json().catch(() => null);
    const message = payload?.error?.message || `请求失败: ${res.status}`;
    throw new Error(message);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

export const api = {
  listEndpoints() {
    return request<Endpoint[]>("/api/endpoints");
  },
  getEndpoint(id: string) {
    return request<Endpoint>(`/api/endpoints/${id}`);
  },
  createEndpoint(payload: Partial<Endpoint>) {
    return request<Endpoint>("/api/endpoints", {
      method: "POST",
      body: JSON.stringify(payload)
    });
  },
  updateEndpoint(id: string, payload: Partial<Endpoint>) {
    return request<Endpoint>(`/api/endpoints/${id}`, {
      method: "PUT",
      body: JSON.stringify(payload)
    });
  },
  deleteEndpoint(id: string) {
    return request<void>(`/api/endpoints/${id}`, { method: "DELETE" });
  },
  listRules(endpointId: string) {
    return request<Rule[]>(`/api/endpoints/${endpointId}/rules`);
  },
  createRule(endpointId: string, payload: Partial<Rule>) {
    return request<Rule>(`/api/endpoints/${endpointId}/rules`, {
      method: "POST",
      body: JSON.stringify(payload)
    });
  },
  updateRule(id: string, payload: Partial<Rule>) {
    return request<Rule>(`/api/rules/${id}`, {
      method: "PUT",
      body: JSON.stringify(payload)
    });
  },
  deleteRule(id: string) {
    return request<void>(`/api/rules/${id}`, { method: "DELETE" });
  },
  preview(payload: PreviewRequest) {
    return request<PreviewResponse>("/api/preview", {
      method: "POST",
      body: JSON.stringify(payload)
    });
  },
  exportAll() {
    return request<ExportBundle>("/api/export");
  },
  importAll(payload: ExportBundle) {
    return request<void>("/api/import", {
      method: "POST",
      body: JSON.stringify(payload)
    });
  }
};
