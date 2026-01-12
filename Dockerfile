#
# 多阶段构建：先构建前端 web/dist，再构建 Go 二进制，最后打包运行镜像。
#

FROM node:20-alpine AS web-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS backend-builder
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/mock-server .

FROM alpine:3.20
WORKDIR /app
RUN addgroup -S app && adduser -S app -G app

COPY --from=backend-builder /out/mock-server ./mock-server
COPY --from=web-builder /src/web/dist ./web/dist

ENV MOCK_ADDR=0.0.0.0:8180
ENV ADMIN_ADDR=0.0.0.0:8181
ENV DB_PATH=/data/mock.db

RUN mkdir -p /data && chown -R app:app /data /app
USER app

EXPOSE 8180 8181
CMD ["./mock-server"]
