# mock-server

一个带可视化管理台的 Mock Server（Go/Gin + Vue3/Vite）。  
管理台（管理端口）默认首页：`/`，管理 API：`/api/*`；Mock 流量在独立的 Mock 端口提供。

## 运行（Docker Compose）

1. 启动：
   - `docker compose up -d --build`
2. 访问：
   - Mock 服务：`http://localhost:8180/`
   - 管理台：`http://localhost:8181/`
3. 数据持久化：
   - SQLite 文件默认位于容器的 `/data/mock.db`，由 compose volume `mock-server-data` 持久化。

可选开启管理台 BasicAuth（同时设置才会启用）：
- `ADMIN_USER=admin ADMIN_PASS=admin docker compose up -d --build`

## 运行（Docker）

- 构建：`docker build -t mock-server:local .`
- 启动：
  - `docker run --rm -p 8180:8180 -p 8181:8181 -v mock-server-data:/data mock-server:local`
  - 可选鉴权：`-e ADMIN_USER=admin -e ADMIN_PASS=admin`

## 本地开发

### 后端

- `make backend-run`
- 环境变量：
  - `MOCK_ADDR`：Mock 端口监听地址，默认 `127.0.0.1:8180`
  - `ADMIN_ADDR`：管理端口监听地址，默认 `127.0.0.1:8181`
  - `DB_PATH`：SQLite 路径，默认 `data/mock.db`
  - `ADMIN_USER`/`ADMIN_PASS`：同时设置则启用管理端口 BasicAuth（包含 `/` 与 `/api/*`）

如果你想用仓库内的 `.env`：
- `cp .env.example .env && source .env && make backend-run`

### 前端

- `make web-dev`
- Vite 默认代理管理端口到 `http://127.0.0.1:8181`（见 `web/vite.config.ts`）

说明：
- 前端开发时请访问 `http://127.0.0.1:5173/`
- 管理端口 `http://127.0.0.1:8181/` 只有在构建出 `web/dist` 后才会有页面（`make web-build` 或 Docker 构建时自动生成）

### 生产式（后端直接托管管理台静态资源）

1. 构建前端：`make web-build`（生成 `web/dist`）
2. 启动后端：`make backend-run`
3. 访问：`http://127.0.0.1:8181/`
