.PHONY: help backend-run backend-test backend-tidy web-dev web-build web-preview

help:
	@echo "Targets:"
	@echo "  backend-run    - 运行后端服务"
	@echo "  backend-test   - 运行后端测试"
	@echo "  backend-tidy   - 运行 go mod tidy"
	@echo "  web-dev        - 启动前端开发服务器"
	@echo "  web-build      - 构建前端"
	@echo "  web-preview    - 预览前端构建"

backend-run:
	cd backend && go run .

backend-test:
	cd backend && go test ./...

backend-tidy:
	cd backend && go mod tidy

web-dev:
	cd web && npm install && npm run dev

web-build:
	cd web && npm install && npm run build

web-preview:
	cd web && npm install && npm run preview
