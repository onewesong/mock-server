# Mock Server（Vue3 + Gin）设计方案

目标：提供一个可视化的 Mock 服务器管理台。左侧可浏览/选择/新增 endpoint；右侧配置该 endpoint 的触发条件（匹配规则）与 Mock 响应；保存后立即生效，对外提供实际的 Mock 接口服务。

## 1. 总体架构

- **单仓库（monorepo）**：Go 后端 + Vue3 前端。
- **运行时分区**：
  - 管理台 UI：`/__admin/`（SPA）
  - 管理 API：`/__admin/api/*`
  - Mock 流量：除 `__admin` 前缀外的所有请求（或可配置前缀，如 `/mock/*`）
- **核心思想**：Gin 只注册管理路由；Mock 请求走 `NoRoute` 统一入口，由“规则引擎”在内存中匹配 endpoint/rule，渲染响应并返回。

## 2. 关键需求与交互

### 2.1 用户操作流

1) 左侧列表：搜索/分组（按 Tag/文件夹）/选择 endpoint。  
2) 新增 endpoint：填写 `method + pathPattern + 名称/描述 + tag`，创建后自动选中。  
3) 右侧编辑区（Tabs 或分区）：
   - 请求匹配：Path 参数、Query/Header/Cookie/Body 条件
   - 规则（场景）：优先级、启用开关、命中策略（第一个命中/按权重随机）
   - 响应构造：状态码、响应头、延迟、响应体（JSON/文本）
   - 预览/调试：输入一份“模拟请求”，后端返回“将命中的规则 + 渲染后的响应预览 + 命中原因”
4) 保存：前端调用管理 API 持久化；后端更新内存索引，立即对 Mock 请求生效。

### 2.2 必备能力（MVP）

- Endpoint CRUD、规则（Rule/Scenario）CRUD
- 基础触发条件：method + pathPattern +（可选）query/header/body 条件
- 基础响应：status + headers + body + delay
- 导入/导出（JSON）
- 管理台基础鉴权（可选开关）

## 3. 数据模型（建议）

### 3.1 概念

- **Project**：一个 Mock 项目（可选；MVP 可先做单项目）
- **Endpoint**：对外暴露的“接口定义”（method + pathPattern）
- **Rule（场景）**：同一 endpoint 下的多套命中条件与对应响应（按优先级/权重选择）

### 3.2 Endpoint（示例字段）

- `id`：UUID/ULID
- `name`：展示名
- `method`：GET/POST/PUT/DELETE/...
- `pathPattern`：如 `/users/:id`、`/orders/:orderId/items`
- `enabled`：是否启用
- `tags`：`["user","v1"]`
- `description`
- `createdAt/updatedAt`

### 3.3 Rule（示例字段）

- `id`
- `endpointId`
- `name`
- `enabled`
- `priority`：数字越小优先级越高（或反之，需统一）
- `weight`：用于“按权重随机”的命中策略（默认 1）
- `matchers`：匹配器数组（见 3.4）
- `response`：响应配置（见 3.5）

### 3.4 Matcher（触发条件）

每个 matcher 表示一个原子条件；Rule 的命中规则：默认 **AND**（全部满足）。

- `source`：`pathParam | query | header | cookie | bodyJsonPath | bodyRaw | method`
- `key`：如 `id`、`X-Env`、`$.data.userId`
- `op`：`eq | ne | contains | regex | in | exists`
- `value`：字符串/数组（按 op 解释）
- `caseSensitive`：可选

说明：
- `pathParam` 来自 `pathPattern` 的参数（如 `:id`）
- `bodyJsonPath` 仅当请求体是 JSON 时生效；解析失败则 matcher 失败

### 3.5 Response（Mock 响应）

- `status`：默认 200
- `headers`：键值对
- `delayMs`：人工延迟
- `bodyType`：`json | text`
- `body`：字符串（json/text）
- `contentType`：可选（不填则按 bodyType 推导）

说明：本次实现不启用模板响应；后续如需可在 M2/M3 增加。

## 4. 后端设计（Go + Gin）

### 4.1 目录结构（建议）

```
/
  backend/              # Go Gin 服务
  web/                  # Vue3 管理台
  DESIGN.md
```

### 4.2 路由与职责

- 管理 API（仅管理台使用）：
  - `GET /__admin/api/endpoints`
  - `POST /__admin/api/endpoints`
  - `GET /__admin/api/endpoints/:id`
  - `PUT /__admin/api/endpoints/:id`
  - `DELETE /__admin/api/endpoints/:id`
  - `GET /__admin/api/endpoints/:id/rules`
  - `POST /__admin/api/endpoints/:id/rules`
  - `PUT /__admin/api/rules/:id`
  - `DELETE /__admin/api/rules/:id`
  - `POST /__admin/api/preview`：输入“模拟请求”，返回命中规则与渲染结果
  - `GET /__admin/api/export`、`POST /__admin/api/import`
- 管理台静态资源：
  - `GET /__admin/*`（SPA history fallback）
- Mock 请求入口：
  - `NoRoute(mockHandler)`：除 `__admin` 前缀以外的所有请求

#### 管理 API：请求/响应约定（建议）

- 统一错误格式：
  - `{"error":{"code":"VALIDATION_ERROR","message":"...","details":[...]}}`
- `POST /__admin/api/preview`（示例请求）：
  - `{"method":"POST","path":"/users/123","query":{"debug":"1"},"headers":{"X-Env":"test"},"body":"{\"role\":\"admin\"}"}`
- `POST /__admin/api/preview`（示例响应）：
  - `{"matched":true,"endpointId":"...","ruleId":"...","explain":["method ok","path ok","header X-Env=... ok"],"response":{"status":200,"headers":{"Content-Type":"application/json"},"body":"...","delayMs":50}}`

### 4.3 规则引擎（匹配流程）

1) 过滤候选 endpoint：`enabled=true` 且 method 匹配  
2) pathPattern 匹配：
   - 支持参数/通配/正则三类：
     - `:param`：命名参数，匹配单个路径段（不含 `/`），如 `/users/:id`
     - `*`：匿名通配，匹配单个路径段，如 `/files/*/meta`
     - `**`：多段通配，匹配剩余路径（可包含 `/`），建议仅允许出现在末尾，如 `/assets/**`
     - `re:<regex>`：正则模式，整条 path 用正则匹配（例如 `re:^/users/\\d+$`）
   - 运行时将 `pathPattern` 预编译为正则，并提取 `:param` 对应的 pathParams（`re:` 模式不自动提取参数，后续可扩展支持命名捕获组）
3) 对候选 endpoint 的 rules 按 `priority` 排序，依次检查：
   - `enabled=true`
   - matchers 全部满足（默认 AND）
4) 命中策略：
   - 默认：第一个命中的 rule
   - 可选：同 priority 的多个 rule 按 weight 随机
5) 生成响应：
   - delay
   - 写入 status/headers/body

未命中行为（建议）：
- 返回 `404`，并可选通过配置开启“调试信息响应头”（例如 `X-Mock-Reason: no-rule-matched`），默认关闭以避免泄露内部规则。

性能策略（MVP 即可达标）：
- 启动或数据变更时构建内存索引：`method -> []compiledEndpoint`
- `compiledEndpoint` 内包含：path 正则、参数名、rules（含预编译 regex/jsonpath）

### 4.4 持久化

本方案默认使用 **SQLite** 作为持久化（你已确认）。

建议：
- 数据库文件：`data/mock.db`（可配置）
- 启动加载：从 SQLite 加载全部 endpoints/rules 到内存索引
- 变更写入：管理 API 写入 SQLite（事务）后，触发内存索引重建/增量更新

#### SQLite 表结构（建议最小化）

`endpoints`
- `id TEXT PRIMARY KEY`
- `name TEXT`
- `method TEXT NOT NULL`
- `path_pattern TEXT NOT NULL`
- `enabled INTEGER NOT NULL`（0/1）
- `tags_json TEXT NOT NULL`（JSON 数组字符串）
- `description TEXT`
- `created_at INTEGER NOT NULL`（unix ms）
- `updated_at INTEGER NOT NULL`（unix ms）

`rules`
- `id TEXT PRIMARY KEY`
- `endpoint_id TEXT NOT NULL`（外键逻辑约束即可）
- `name TEXT`
- `enabled INTEGER NOT NULL`
- `priority INTEGER NOT NULL`
- `weight INTEGER NOT NULL`
- `matchers_json TEXT NOT NULL`（Matcher 数组 JSON）
- `response_json TEXT NOT NULL`（Response JSON）
- `created_at INTEGER NOT NULL`
- `updated_at INTEGER NOT NULL`

索引建议：
- `CREATE INDEX idx_rules_endpoint_id ON rules(endpoint_id);`

### 4.5 配置与安全

- 默认仅监听 `127.0.0.1`（可通过环境变量改为 `0.0.0.0`）
- 管理台鉴权（默认启用 Basic Auth，你已确认）：
  - 仅保护 `__admin` 静态资源与 `__admin/api/*`
  - 用户名/密码通过环境变量配置（例如 `ADMIN_USER`/`ADMIN_PASS`）
- CORS：仅对 `__admin` API 允许（开发时支持 Vite 代理）
- 预留：请求审计日志（命中哪个 endpoint/rule）

## 5. 前端设计（Vue3）

### 5.1 技术栈建议

- Vue 3 + TypeScript + Vite
- 状态管理：Pinia
- 路由：Vue Router（可选；MVP 可单页）
- UI：Element Plus（表单/表格/弹窗成熟）
- 编辑器：
  - JSON：Monaco Editor（或 CodeMirror 6）
  - 支持 JSON 格式化与校验提示

### 5.2 页面布局

- 顶部：项目名、导入/导出、全局保存状态、（可选）运行状态提示
- 左侧（Endpoint 列表）：
  - 搜索框
  - 分组/标签筛选
  - 列表项展示：`METHOD` 彩色标签 + `pathPattern` + 名称
  - 新增按钮（弹窗/抽屉）
- 右侧（编辑区）：
  - Endpoint 基本信息（method/pathPattern/name/tags/enabled）
  - Rules 列表（可拖拽排序/设置 priority；MVP 可先用数字）
  - Rule 编辑：
    - Matchers 表格：source/key/op/value
    - Response 表单：status/headers/delay/bodyType/body
  - 调试预览：提交模拟请求，展示命中结果与响应预览

### 5.3 前端数据流

- `useEndpointsStore`：
  - `endpoints[]`、`selectedEndpointId`、`selectedEndpointDetail`
  - `dirty`（本地未保存变更）
  - actions：`fetchList/fetchDetail/saveEndpoint/saveRule/delete.../preview`
- API 客户端：统一封装错误提示、loading、请求取消（切换 endpoint 时）

## 6. 里程碑（建议）

- M1（可用版）：Endpoint+Rule CRUD、基础匹配（method/path/query/header/bodyJsonPath）、基础响应、立即生效、导入导出
- M2（体验版）：模板响应、规则命中预览更完善、请求命中历史列表
- M3（高级）：代理转发（先 proxy 再改写）、状态化 Mock（序列/计数器）、团队协作与多项目

## 7. 校验、兼容与扩展点

### 7.1 后端校验（建议最少集）

- `method` 必填，且为允许集合（GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS）
- `pathPattern` 必填：
  - 必须以 `/` 开头
  - 不能以 `/_` 或 `/__admin` 开头（避免与管理台冲突）
  - `:param` 名称需满足 `[A-Za-z_][A-Za-z0-9_]*`
  - `**` 若支持则建议仅允许在末尾；`re:` 模式的正则必须可编译
- `priority` 为整数（MVP 可限定范围，如 0~10000）
- `matchers`：
  - `regex` 必须可编译
  - `bodyJsonPath` 必须可编译（或首次使用时编译并缓存）
- `response`：
  - `status` 取值 100~599
  - `headers` key/value 必须为可打印字符
  - `bodyType=json` 时 `body` 必须是合法 JSON（便于前端与预览）

### 7.2 兼容性约定

- 请求体解析优先级：JSON > form > raw（仅用于 matcher/模板上下文；不改变原始 body）
- 注：本次不启用模板响应，上述“模板上下文”相关内容仅作为后续扩展预留
- 同一 endpoint 下多个 rule 命中时的确定性：按 `priority` +（次序或 weight 策略）决定

### 7.3 扩展点（先预留接口，不一定 M1 实现）

- **Proxy**：未命中时转发到上游；或命中后“基于上游响应改写”
- **State**：计数器/序列（第 N 次命中返回不同响应）
- **变量提取**：从 regex/jsonpath 提取命名变量供模板使用
- **请求历史**：最近 N 条命中记录（用于 UI 回放与调试）
- **模板响应**：M2/M3 可引入 Go `text/template`（本次不启用）

## 8. 需要你确认的关键决策（实现前必须定）

已确认：
1) Mock 接口暴露路径：全局 `NoRoute`  
2) 持久化方式：SQLite  
3) pathPattern 语法：支持 `:param` + `*`/`**` + `re:<regex>`  
4) 管理台鉴权：默认 Basic Auth  
5) 模板响应：不启用（仅 `json/text`）
