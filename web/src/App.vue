<template>
  <div class="app-shell">
    <header class="app-header">
      <div>
        <div class="app-title">Mock Server 管理台</div>
        <div class="app-subtitle">endpoint 与规则配置即时生效</div>
      </div>
      <div class="actions-row">
        <el-button type="primary" @click="openCreateDialog">新增 Endpoint</el-button>
        <el-button @click="exportData">导出</el-button>
        <el-upload :auto-upload="false" :show-file-list="false" accept="application/json" @change="importData">
          <el-button>导入</el-button>
        </el-upload>
      </div>
    </header>

    <div class="app-grid">
      <section class="panel">
        <div class="panel-title">Endpoints</div>
        <el-input v-model="search" placeholder="搜索路径或名称" clearable />
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
            <div class="endpoint-meta" v-if="ep.tags?.length">标签：{{ ep.tags.join(", ") }}</div>
          </div>
        </div>
      </section>

      <section class="panel" v-if="store.selected">
        <div class="panel-title">Endpoint 配置</div>
        <el-form :model="endpointForm" label-width="90px">
          <el-form-item label="启用">
            <el-switch v-model="endpointForm.enabled" />
          </el-form-item>
          <el-form-item label="方法">
            <el-select v-model="endpointForm.method" placeholder="选择方法">
              <el-option v-for="m in methods" :key="m" :label="m" :value="m" />
            </el-select>
          </el-form-item>
          <el-form-item label="路径">
            <el-input v-model="endpointForm.pathPattern" placeholder="/users/:id" />
          </el-form-item>
          <el-form-item label="名称">
            <el-input v-model="endpointForm.name" />
          </el-form-item>
          <el-form-item label="标签">
            <el-input v-model="tagsInput" placeholder="用英文逗号分隔" />
          </el-form-item>
          <el-form-item label="描述">
            <el-input v-model="endpointForm.description" type="textarea" :rows="2" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="saveEndpoint">保存 Endpoint</el-button>
            <el-button type="danger" plain @click="removeEndpoint">删除</el-button>
          </el-form-item>
        </el-form>

        <div class="panel-title" style="margin-top: 24px">Rules</div>
        <div class="actions-row" style="margin-bottom: 12px">
          <el-button type="primary" @click="openRuleDialog()">新增 Rule</el-button>
        </div>
        <el-table :data="store.rules" size="small">
          <el-table-column prop="name" label="名称" min-width="140" />
          <el-table-column prop="priority" label="优先级" width="90" />
          <el-table-column prop="weight" label="权重" width="80" />
          <el-table-column prop="enabled" label="启用" width="70">
            <template #default="scope">
              <el-tag v-if="scope.row.enabled" type="success">启用</el-tag>
              <el-tag v-else type="info">关闭</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="140">
            <template #default="scope">
              <el-button size="small" @click="openRuleDialog(scope.row)">编辑</el-button>
              <el-button size="small" type="danger" plain @click="removeRule(scope.row.id)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="panel-title" style="margin-top: 24px">调试预览</div>
        <el-form :model="previewForm" label-width="90px">
          <el-form-item label="方法">
            <el-select v-model="previewForm.method" placeholder="GET">
              <el-option v-for="m in methods" :key="m" :label="m" :value="m" />
            </el-select>
          </el-form-item>
          <el-form-item label="路径">
            <el-input v-model="previewForm.path" placeholder="/users/123" />
          </el-form-item>
          <el-form-item label="Query">
            <el-input v-model="previewQuery" placeholder='{"debug":"1"}' />
          </el-form-item>
          <el-form-item label="Headers">
            <el-input v-model="previewHeaders" placeholder='{"X-Env":"test"}' />
          </el-form-item>
          <el-form-item label="Body">
            <el-input v-model="previewForm.body" type="textarea" :rows="3" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="runPreview">预览命中</el-button>
          </el-form-item>
        </el-form>
        <el-card v-if="store.preview" style="margin-top: 12px">
          <div>Matched: {{ store.preview.matched }}</div>
          <div v-if="store.preview.endpointId">Endpoint: {{ store.preview.endpointId }}</div>
          <div v-if="store.preview.ruleId">Rule: {{ store.preview.ruleId }}</div>
          <div style="margin-top: 8px; color: var(--muted)">
            {{ store.preview.explain?.join("; ") }}
          </div>
          <pre v-if="store.preview.response" style="margin-top: 8px; white-space: pre-wrap">
{{ JSON.stringify(store.preview.response, null, 2) }}
          </pre>
        </el-card>
      </section>
    </div>

    <el-dialog v-model="showEndpointDialog" title="新增 Endpoint" width="520px">
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
          <el-input v-model="createForm.name" />
        </el-form-item>
        <el-form-item label="标签">
          <el-input v-model="createTags" placeholder="用英文逗号分隔" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="createForm.description" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEndpointDialog = false">取消</el-button>
        <el-button type="primary" @click="createEndpoint">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showRuleDialog" title="Rule 设置" width="720px">
      <el-form :model="ruleForm" label-width="90px">
        <el-form-item label="名称">
          <el-input v-model="ruleForm.name" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="ruleForm.enabled" />
        </el-form-item>
        <el-form-item label="优先级">
          <el-input-number v-model="ruleForm.priority" :min="0" />
        </el-form-item>
        <el-form-item label="权重">
          <el-input-number v-model="ruleForm.weight" :min="0" />
        </el-form-item>
        <el-form-item label="匹配条件">
          <el-table :data="ruleForm.matchers" size="small" class="matchers-table">
            <el-table-column label="来源" width="120">
              <template #default="scope">
                <el-select v-model="scope.row.source" placeholder="source">
                  <el-option v-for="s in matcherSources" :key="s" :label="s" :value="s" />
                </el-select>
              </template>
            </el-table-column>
            <el-table-column label="Key" width="140">
              <template #default="scope">
                <el-input v-model="scope.row.key" />
              </template>
            </el-table-column>
            <el-table-column label="Op" width="120">
              <template #default="scope">
                <el-select v-model="scope.row.op" placeholder="op">
                  <el-option v-for="o in matcherOps" :key="o" :label="o" :value="o" />
                </el-select>
              </template>
            </el-table-column>
            <el-table-column label="Value">
              <template #default="scope">
                <el-input v-model="scope.row.value" />
              </template>
            </el-table-column>
            <el-table-column label="Case" width="80">
              <template #default="scope">
                <el-switch v-model="scope.row.caseSensitive" />
              </template>
            </el-table-column>
            <el-table-column label="操作" width="80">
              <template #default="scope">
                <el-button size="small" type="danger" plain @click="removeMatcher(scope.$index)">删</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-button size="small" style="margin-top: 8px" @click="addMatcher">新增条件</el-button>
        </el-form-item>
        <el-form-item label="状态码">
          <el-input-number v-model="ruleForm.response.status" :min="100" :max="599" />
        </el-form-item>
        <el-form-item label="延迟(ms)">
          <el-input-number v-model="ruleForm.response.delayMs" :min="0" />
        </el-form-item>
        <el-form-item label="Body 类型">
          <el-select v-model="ruleForm.response.bodyType">
            <el-option label="json" value="json" />
            <el-option label="text" value="text" />
          </el-select>
        </el-form-item>
        <el-form-item label="Headers">
          <el-input v-model="headersInput" placeholder='{"Content-Type":"application/json"}' />
        </el-form-item>
        <el-form-item label="Body">
          <el-input v-model="ruleForm.response.body" type="textarea" :rows="4" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showRuleDialog = false">取消</el-button>
        <el-button type="primary" @click="saveRule">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { useEndpointsStore } from "./stores/endpoints";
import type { Endpoint, Rule } from "./types";
import { api } from "./api";

const store = useEndpointsStore();
const search = ref("");
const showEndpointDialog = ref(false);
const showRuleDialog = ref(false);

const methods = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"];
const matcherSources = ["pathParam", "query", "header", "cookie", "bodyJsonPath", "bodyRaw", "method"];
const matcherOps = ["eq", "ne", "contains", "regex", "in", "exists"];

const createForm = reactive<Partial<Endpoint>>({
  method: "GET",
  pathPattern: "",
  name: "",
  description: "",
  enabled: true,
  tags: []
});
const createTags = ref("");

const endpointForm = reactive<Partial<Endpoint>>({
  method: "GET",
  pathPattern: "",
  name: "",
  description: "",
  enabled: true,
  tags: []
});
const tagsInput = ref("");

const ruleForm = reactive<Partial<Rule>>({
  id: "",
  name: "",
  enabled: true,
  priority: 0,
  weight: 1,
  matchers: [],
  response: {
    status: 200,
    headers: {},
    delayMs: 0,
    bodyType: "json",
    body: "",
    contentType: ""
  }
});
const editingRuleId = ref("");
const headersInput = ref("{}");

const previewForm = reactive({
  method: "GET",
  path: "",
  body: ""
});
const previewQuery = ref("{}");
const previewHeaders = ref("{}");

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
    endpointForm.description = val.description;
    endpointForm.enabled = val.enabled;
    endpointForm.tags = val.tags || [];
    tagsInput.value = (val.tags || []).join(",");
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
  createTags.value = "";
  showEndpointDialog.value = true;
}

async function createEndpoint() {
  try {
    const tags = createTags.value
      .split(",")
      .map((t) => t.trim())
      .filter(Boolean);
    await store.createEndpoint({ ...createForm, tags, enabled: true });
    showEndpointDialog.value = false;
  } catch (err: any) {
    ElMessage.error(err.message);
  }
}

async function saveEndpoint() {
  if (!store.selected) return;
  try {
    const tags = tagsInput.value
      .split(",")
      .map((t) => t.trim())
      .filter(Boolean);
    await store.updateEndpoint(store.selected.id, { ...endpointForm, tags });
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

function openRuleDialog(rule?: Rule) {
  if (rule) {
    editingRuleId.value = rule.id;
    ruleForm.id = rule.id;
    ruleForm.name = rule.name;
    ruleForm.enabled = rule.enabled;
    ruleForm.priority = rule.priority;
    ruleForm.weight = rule.weight;
    ruleForm.matchers = rule.matchers.map((m) => ({ ...m }));
    ruleForm.response = { ...rule.response, headers: { ...rule.response.headers } };
    headersInput.value = JSON.stringify(rule.response.headers || {});
  } else {
    editingRuleId.value = "";
    ruleForm.id = "";
    ruleForm.name = "";
    ruleForm.enabled = true;
    ruleForm.priority = 0;
    ruleForm.weight = 1;
    ruleForm.matchers = [];
    ruleForm.response = {
      status: 200,
      headers: {},
      delayMs: 0,
      bodyType: "json",
      body: "",
      contentType: ""
    };
    headersInput.value = "{}";
  }
  showRuleDialog.value = true;
}

function addMatcher() {
  ruleForm.matchers?.push({
    source: "query",
    key: "",
    op: "eq",
    value: "",
    caseSensitive: false
  });
}

function removeMatcher(index: number) {
  ruleForm.matchers?.splice(index, 1);
}

async function saveRule() {
  try {
    const headers = JSON.parse(headersInput.value || "{}");
    ruleForm.response = { ...ruleForm.response, headers };
    if (editingRuleId.value) {
      await store.updateRule(editingRuleId.value, ruleForm);
    } else {
      await store.createRule(ruleForm);
    }
    showRuleDialog.value = false;
  } catch (err: any) {
    ElMessage.error(err.message || "保存失败");
  }
}

async function removeRule(id: string) {
  try {
    await store.deleteRule(id);
  } catch (err: any) {
    ElMessage.error(err.message);
  }
}

async function runPreview() {
  try {
    const query = JSON.parse(previewQuery.value || "{}");
    const headers = JSON.parse(previewHeaders.value || "{}");
    await store.previewRequest({
      method: previewForm.method,
      path: previewForm.path,
      query,
      headers,
      body: previewForm.body
    });
  } catch (err: any) {
    ElMessage.error(err.message || "预览失败");
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
