# kccol-token-hub 仓库结构说明

本文用于快速了解当前仓库的目录组织和各模块职责，便于新成员、协作开发和后续维护。

## 仓库概览

`kccol-token-hub` 是一个以 Go 为核心的服务端项目，整体采用分层结构组织代码，同时包含前端静态资源、数据库迁移脚本、部署配置和多语言文档。并且作为子仓在kccolhub的单体服务micro-service-kccol-token-hub中

## 顶层目录

```text
kccol-token-hub/
├── main.go                  # 程序入口
├── go.mod / go.sum          # Go 依赖管理
├── Dockerfile*               # 容器构建配置
├── docker-compose*.yml       # 本地/开发部署编排
├── makefile                  # 常用构建与维护命令
├── README*.md                # 多语言项目说明文档
├── VERSION                   # 当前版本号
├── LICENSE / NOTICE         # 开源许可与声明
├── THIRD-PARTY-LICENSES.md   # 第三方依赖许可汇总
├── AGENTS.md / CLAUDE.md     # 协作与代理说明
├── bin/                      # 数据库迁移与辅助脚本
├── common/                   # 通用工具与基础能力
├── constant/                 # 常量定义
├── controller/               # HTTP 控制层
├── docs/                     # 文档与说明
├── dto/                      # 数据传输对象
├── electron/                 # 桌面端相关资源
├── i18n/                     # 后端国际化资源
├── logger/                   # 日志相关封装
├── middleware/               # 中间件
├── model/                    # 数据模型与数据库访问
├── oauth/                    # 第三方认证实现
├── pkg/                      # 内部复用包
├── relay/                    # 请求转发与上游适配
├── router/                   # 路由注册
├── service/                  # 业务服务层
├── setting/                  # 配置与系统设置
├── types/                    # 类型定义
└── web/                      # 前端资源
```

## 主要模块说明

### `main.go`

程序启动入口，负责加载配置、初始化运行环境并启动服务。

### `router/`

定义 HTTP 路由，是请求进入系统的统一入口。

### `controller/`

负责接收请求、校验参数并调用 service 层处理业务。

### `service/`

承载核心业务逻辑，避免控制层直接处理复杂规则。

### `model/`

封装数据库模型与持久化访问逻辑。

### `relay/`

与上游 AI 服务交互的转发层，通常包含协议适配、请求改写和响应转换。

### `middleware/`

存放认证、限流、日志、跨域等通用中间件。

### `common/`

放置项目通用能力，例如 JSON、加密、缓存、校验、工具函数等。

### `constant/`

集中维护枚举、键名、状态值等固定常量。

### `dto/`

用于接口请求和响应的数据结构定义。

### `oauth/`

各类 OAuth/OIDC 登录相关实现。

### `setting/`

系统配置、运行参数和后台设置项相关代码。

### `pkg/`

项目内部可复用的功能包。

### `web/`

前端页面资源。当前仓库中包含：

- `web/default/`：默认前端
- `web/classic/`：经典前端

## 支撑文件与目录

### `bin/`

包含数据库迁移脚本和辅助工具脚本。

### `docs/`

保存补充文档、示意图和专题说明。

### `i18n/`

后端国际化资源目录。

### `electron/`

桌面端相关资源或打包支持文件。

### `docker-compose.yml` / `Dockerfile`

用于本地开发、测试和生产部署。

### `README*.md`

仓库说明文档的多语言版本，便于不同语言使用者快速上手。

## 阅读建议

如果你想快速定位某块功能，可以按下面顺序看代码：

1. 先看 `router/`，了解请求入口。
2. 再看对应的 `controller/`，了解接口如何分发。
3. 接着看 `service/` 和 `model/`，理解业务与数据处理流程。
4. 如涉及上游调用，再看 `relay/`。
5. 如涉及公共能力，优先查 `common/`、`middleware/`、`setting/`。

## 备注

本文件只描述当前仓库的目录结构，不替代完整的产品文档或接口文档。
