# SubForMe

轻量级 Mihomo/Clash 订阅配置生成系统。不是订阅转换器，而是**配置组合引擎**。

## 解决的问题

通过3x-ui自建 VLESS/VMESS/Trojan 等代理服务后，手动编写 Mihomo 配置繁琐且容易出错。SubForMe 让你：

1. 在 3x-ui 面板管理用户和入站
2. 在 SubForMe 的 Web UI 中**可视化编辑**：
   - VPS 节点（一台服务器一条记录）
   - 代理分组（自定义类型和策略）
   - 为每个用户指定使用哪些节点、分别分配到哪个组
   - 基础模板（DNS、TUN、规则等）
3. 每个用户获得专属订阅链接，导入 Mihomo/Clash Verge/OpenClash 即可用
4. 同步主面板的用户配置到多个面板

## 核心概念

```
固定基础配置（base.yaml）
+
从 3x-ui API 动态获取用户节点信息
+
Web UI 中配置的节点/分组/策略
=
最终完整的 Mihomo config.yaml，生成订阅地址
```

## 快速开始

### Docker 运行

```bash
docker run -d \
  --name subforme \
  -p 8080:8080 \
  -v ./config:/app/config \
  charley008/subforme:latest
```

> 默认管理员账号 `admin` / `123456`，建议登录后修改密码。3x-ui 面板配置通过 Web UI 添加。

或者使用 docker-compose：
```bash
docker compose up -d
```

### 直接运行

下载对应平台的 release，解压后修改 `config/config.json`：

```json
{
  "listen": ":8080",
  "admin_username": "admin",
  "admin_password": "123456",
  "session_secret": "random-secret",
  "config_dir": ".",
  "frontend_dir": "../web"
}
```

运行：
```bash
./subforme
```

打开 `http://your-server:8080` 登录。

## 首次使用流程

```
1. 服务器 → 添加你的 3x-ui 面板（名称、地址、API Key）
2. 节点 → 添加 VPS 机器，并关联到对应服务器
3. 代理分组 → 定义分组（手动选择、自动测速、第三方等）
4. 用户 → 从主面板导入用户 → 勾选节点 → 分组编辑器分配节点
5. 用户 → 点击预览或复制订阅链接
6. 多面板同步 → 添加其他面板 → 点击"同步到非主面板服务器"
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `SUBFORME_ADMIN_USERNAME` | 后台管理员用户名 | `admin` |
| `SUBFORME_ADMIN_PASSWORD` | 后台管理员密码 | `123456` |
| `SUBFORME_SESSION_SECRET` | Session 加密密钥 | - |
| `SUBFORME_CONFIG_DIR` | 配置目录 | `/app/config` |
| `SUBFORME_FRONTEND_DIR` | 前端静态文件目录 | `/app/web` |
| `SUBFORME_LISTEN` | 监听地址 | `:8080` |

## Nginx 反代

### 子域名方式（推荐）

```nginx
server {
    listen 443 ssl;
    server_name sub.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

### 子路径方式

```nginx
server {
    listen 443 ssl;
    server_name example.com;

    location = /subforme {
        return 301 /subforme/;
    }
    location /subforme/ {
        proxy_pass http://127.0.0.1:8080/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

> 订阅链接格式：`https://example.com/api/sub?user=xxx`

## 本地构建

### 构建前端

```bash
cd frontend
npm install
npm run build
```

### 构建后端

**Windows:**
```bash
cd backend
go build -o ../release/subforme.exe ./cmd/subforme
```

**Linux:**
```bash
cd backend
GOOS=linux GOARCH=amd64 go build -o ../release/subforme-linux-x86_64 ./cmd/subforme
```

### Docker 镜像

```bash
docker build -t subforme:latest .
```

## 目录结构

```
subforme/
├── backend/          # Go 后端
│   ├── cmd/subforme/ # 入口
│   ├── internal/     # 业务逻辑
│   │   ├── app/      # 服务层
│   │   ├── config/   # 配置管理
│   │   ├── db/       # SQLite 数据库层
│   │   ├── generator/# YAML 生成器
│   │   ├── groups/   # 代理分组
│   │   ├── web/      # HTTP API
│   │   └── xui/      # 3x-ui API 客户端
│   └── config/       # 默认配置
├── frontend/         # React + Vite 前端
└── release/          # 构建产物
```

## 技术栈

- **后端**: Go 1.26 + SQLite
- **前端**: React 19 + Vite
- **配置**: YAML + SQLite
- **API**: 3x-ui REST API

## License

MIT
