import { defineStore } from "pinia";
import { api } from "../api";
import type { Endpoint, Rule, PreviewRequest, PreviewResponse } from "../types";

export const useEndpointsStore = defineStore("endpoints", {
  state: () => ({
    endpoints: [] as Endpoint[],
    selectedId: "",
    selected: null as Endpoint | null,
    rules: [] as Rule[],
    loading: false,
    preview: null as PreviewResponse | null
  }),
  actions: {
    async loadEndpoints() {
      this.loading = true;
      try {
        this.endpoints = await api.listEndpoints();
        if (!this.selectedId && this.endpoints.length > 0) {
          this.selectEndpoint(this.endpoints[0].id);
        }
      } finally {
        this.loading = false;
      }
    },
    async selectEndpoint(id: string) {
      this.selectedId = id;
      this.selected = await api.getEndpoint(id);
      this.rules = await api.listRules(id);
    },
    async createEndpoint(payload: Partial<Endpoint>) {
      const created = await api.createEndpoint(payload);
      this.endpoints.unshift(created);
      await this.selectEndpoint(created.id);
    },
    async updateEndpoint(id: string, payload: Partial<Endpoint>) {
      const updated = await api.updateEndpoint(id, payload);
      this.selected = updated;
      const idx = this.endpoints.findIndex((ep) => ep.id === id);
      if (idx >= 0) {
        this.endpoints[idx] = updated;
      }
    },
    async deleteEndpoint(id: string) {
      await api.deleteEndpoint(id);
      this.endpoints = this.endpoints.filter((ep) => ep.id !== id);
      if (this.selectedId === id) {
        this.selectedId = "";
        this.selected = null;
        this.rules = [];
      }
    },
    async createRule(payload: Partial<Rule>) {
      if (!this.selectedId) return;
      const created = await api.createRule(this.selectedId, payload);
      this.rules.unshift(created);
    },
    async updateRule(id: string, payload: Partial<Rule>) {
      const updated = await api.updateRule(id, payload);
      const idx = this.rules.findIndex((r) => r.id === id);
      if (idx >= 0) {
        this.rules[idx] = updated;
      }
    },
    async deleteRule(id: string) {
      await api.deleteRule(id);
      this.rules = this.rules.filter((r) => r.id !== id);
    },
    async previewRequest(payload: PreviewRequest) {
      this.preview = await api.preview(payload);
    }
  }
});
