<template>
  <div class="app-shell">
    <header class="app-header">
      <div>
        <div class="app-title">Mock Server</div>
        <div class="app-subtitle">简化版：只配置状态码与响应体</div>
      </div>
      <div class="actions-row">
        <el-button type="primary" @click="openCreateDialog">新增 Route</el-button>
        <el-button @click="exportData">导出</el-button>
        <el-upload :auto-upload="false" :show-file-list="false" accept="application/json" @change="importData">
          <el-button>导入</el-button>
        </el-upload>
      </div>
    </header>

    <div class="app-grid">
      <section class="panel">
        <div class="panel-title">Routes</div>
        <el-input v-model="search" placeholder="过滤（路径/名称）" clearable />
        <div style="margin-top: 12px; display: grid; gap: 8px">
          <div
            v-for="ep in filteredEndpoints"
            :key="ep.id"
            class="endpoint-item"
            :class="{ active: ep.id === store.selectedId }"
            @click="store.selectEndpoint(ep.id)"
          >
            <div class="endpoint-meta">
              <span class="method-pill">{{ ep.method }}</span>
              <span>{{ ep.pathPattern }}</span>
            </div>
            <div>{{ ep.name || "未命名" }}</div>
          </div>
        </div>
      </section>

      <section class="panel" v-if="store.selected">
        <div class="panel-title">Route</div>
        <div class="route-bar">
          <el-switch v-model="endpointForm.enabled" />
          <el-select v-model="endpointForm.method" placeholder="GET" style="width: 120px">
            <el-option v-for="m in methods" :key="m" :label="m" :value="m" />
          </el-select>
          <el-input v-model="endpointForm.pathPattern" placeholder="/users/:id" />
          <el-button type="primary" @click="saveEndpoint">保存</el-button>
          <el-button type="danger" plain @click="removeEndpoint">删除</el-button>
        </div>
        <div style="margin-top: 10px">
          <el-input v-model="endpointForm.name" placeholder="备注/名称（可选）" />
        </div>

        <div class="panel-title" style="margin-top: 20px">Status & Body</div>
        <div v-if="!responseRuleId" class="empty-hint">
          当前 Route 还没有响应配置。
          <el-button type="primary" size="small" @click="createDefaultResponse">创建默认响应</el-button>
        </div>
        <div v-else class="response-editor">
          <div class="response-meta">
            <el-input-number v-model="responseForm.status" :min="100" :max="599" />
            <el-select v-model="responseForm.bodyType" style="width: 120px">
              <el-option label="json" value="json" />
              <el-option label="text" value="text" />
            </el-select>
            <el-button v-if="responseForm.bodyType === 'json'" @click="beautifyJSON">Beautify JSON</el-button>
            <div style="flex: 1"></div>
            <el-button type="primary" @click="saveResponse">保存响应</el-button>
          </div>
          <el-input
            v-model="responseForm.body"
            type="textarea"
            :rows="12"
            placeholder="响应体（json/text）"
            style="margin-top: 10px"
          />
        </div>
      </section>

      <section class="panel" v-else>
        <div class="panel-title">Route</div>
        <div style="color: var(--muted)">先在左侧选择或新增一个 Route</div>
      </section>
    </div>

    <el-dialog v-model="showEndpointDialog" title="新增 Route" width="520px">
      <el-form :model="createForm" label-width="90px">
        <el-form-item label="方法">
          <el-select v-model="createForm.method" placeholder="选择方法">
            <el-option v-for="m in methods" :key="m" :label="m" :value="m" />
          </el-select>
        </el-form-item>
        <el-form-item label="路径">
          <el-input v-model="createForm.pathPattern" placeholder="/users/:id" />
        </el-form-item>
        <el-form-item label="名称">
          <el-input v-model="createForm.name" placeholder="可选" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEndpointDialog = false">取消</el-button>
        <el-button type="primary" @click="createEndpoint">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { useEndpointsStore } from "./stores/endpoints";
import type { Endpoint } from "./types";
import { api } from "./api";

const store = useEndpointsStore();
const search = ref("");
const showEndpointDialog = ref(false);

const methods = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"];

const createForm = reactive<Partial<Endpoint>>({
  method: "GET",
  pathPattern: "",
  name: "",
  description: "",
  enabled: true,
  tags: []
});

const endpointForm = reactive<Partial<Endpoint>>({
  method: "GET",
  pathPattern: "",
  name: "",
  description: "",
  enabled: true,
  tags: []
});

const responseRuleId = ref("");
const responseForm = reactive({
  status: 200,
  bodyType: "json",
  body: "{}",
});

const filteredEndpoints = computed(() => {
  const keyword = search.value.trim().toLowerCase();
  if (!keyword) return store.endpoints;
  return store.endpoints.filter(
    (ep) =>
      ep.pathPattern.toLowerCase().includes(keyword) ||
      (ep.name || "").toLowerCase().includes(keyword)
  );
});

watch(
  () => store.selected,
  (val) => {
    if (!val) return;
    endpointForm.method = val.method;
    endpointForm.pathPattern = val.pathPattern;
    endpointForm.name = val.name;
    endpointForm.enabled = val.enabled;
  },
  { immediate: true }
);

watch(
  () => store.rules,
  () => {
    const primary = store.rules?.[0];
    if (!primary) {
      responseRuleId.value = "";
      responseForm.status = 200;
      responseForm.bodyType = "json";
      responseForm.body = "{}";
      return;
    }
    responseRuleId.value = primary.id;
    responseForm.status = primary.response?.status ?? 200;
    responseForm.bodyType = primary.response?.bodyType || "json";
    responseForm.body = primary.response?.body || "";
  },
  { immediate: true }
);

onMounted(() => {
  store.loadEndpoints().catch((err) => ElMessage.error(err.message));
});

function openCreateDialog() {
  createForm.method = "GET";
  createForm.pathPattern = "";
  createForm.name = "";
  createForm.description = "";
  createForm.enabled = true;
  createForm.tags = [];
  showEndpointDialog.value = true;
}

async function createEndpoint() {
  try {
    await store.createEndpoint({ ...createForm, tags: createForm.tags || [], description: createForm.description || "" });
    showEndpointDialog.value = false;
  } catch (err: any) {
    ElMessage.error(err.message);
  }
}

async function saveEndpoint() {
  if (!store.selected) return;
  try {
    await store.updateEndpoint(store.selected.id, { ...endpointForm, tags: store.selected.tags || [], description: store.selected.description || "" });
    ElMessage.success("已保存");
  } catch (err: any) {
    ElMessage.error(err.message);
  }
}

async function removeEndpoint() {
  if (!store.selected) return;
  try {
    await store.deleteEndpoint(store.selected.id);
    ElMessage.success("已删除");
  } catch (err: any) {
    ElMessage.error(err.message);
  }
}

async function createDefaultResponse() {
  if (!store.selectedId) return;
  try {
    await store.createRule({
      name: "默认响应",
      enabled: true,
      priority: 0,
      weight: 1,
      matchers: [],
      response: {
        status: responseForm.status,
        headers: {},
        delayMs: 0,
        bodyType: responseForm.bodyType,
        body: responseForm.body,
        contentType: ""
      }
    } as any);
    ElMessage.success("已创建");
  } catch (err: any) {
    ElMessage.error(err.message || "创建失败");
  }
}

async function saveResponse() {
  if (!store.selectedId || !responseRuleId.value) return;
  try {
    const rule = store.rules.find((r) => r.id === responseRuleId.value);
    if (!rule) return;

    // 后端校验：json bodyType 且 body 非空时必须是合法 JSON
    if (responseForm.bodyType === "json" && responseForm.body.trim() !== "") {
      JSON.parse(responseForm.body);
    }

    await store.updateRule(rule.id, {
      endpointId: store.selectedId,
      name: rule.name,
      enabled: rule.enabled,
      priority: rule.priority,
      weight: rule.weight,
      matchers: rule.matchers,
      response: {
        ...rule.response,
        status: responseForm.status,
        bodyType: responseForm.bodyType,
        body: responseForm.body
      }
    } as any);
    ElMessage.success("已保存");
  } catch (err: any) {
    ElMessage.error(err.message || "保存失败");
  }
}

function beautifyJSON() {
  try {
    const parsed = JSON.parse(responseForm.body || "{}");
    responseForm.body = JSON.stringify(parsed, null, 2);
  } catch {
    ElMessage.error("JSON 格式不正确");
  }
}

async function exportData() {
  try {
    const bundle = await api.exportAll();
    const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "mock-server-export.json";
    link.click();
    URL.revokeObjectURL(url);
  } catch (err: any) {
    ElMessage.error(err.message || "导出失败");
  }
}

async function importData(fileEvent: any) {
  const file = fileEvent.raw;
  if (!file) return;
  try {
    const text = await file.text();
    const bundle = JSON.parse(text);
    await api.importAll(bundle);
    await store.loadEndpoints();
    ElMessage.success("导入成功");
  } catch (err: any) {
    ElMessage.error(err.message || "导入失败");
  }
}
</script>
