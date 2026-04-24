# Mermaid 图表合集

本文档包含多种 Mermaid 图表示例：时序图、甘特图、流程图和 ER 图。

---

## 1. 时序图 - 用户登录系统

```mermaid
sequenceDiagram
    participant U as 👤 用户
    participant B as 🌐 浏览器
    participant A as 🖥️ 前端服务
    participant Auth as 🔐 认证服务
    participant DB as 🗄️ 数据库

    U->>B: 输入用户名密码
    B->>A: 提交登录请求
    A->>Auth: 转发认证请求
    Auth->>DB: 查询用户信息
    DB-->>Auth: 返回用户数据
    Auth-->>A: 生成 JWT Token
    A-->>B: 返回登录成功 + Token
    B-->>U: 显示用户主页

    Note over U,DB: 登录成功后，用户可以使用Token访问受保护资源
```

**关键步骤：**
1. 用户提交 → 浏览器收集表单数据
2. 前端转发 → 调用认证服务
3. Token 生成 → 使用 JWT 签名
4. 会话建立 → 后续请求携带 Token

---

## 2. 甘特图 - 产品迭代计划

```mermaid
gantt
    title 产品 v2.0 迭代计划
    dateFormat  YYYY-MM-DD

    section 设计阶段
    需求评审           :done,    req1, 2026-04-01, 2026-04-03
    UI/UX 设计         :active,  design, 2026-04-03, 2026-04-10
    技术方案设计        :         tech, 2026-04-06, 2026-04-10

    section 开发阶段
    前端开发           :         fe,    2026-04-11, 2026-04-22
    后端接口开发        :         be,    2026-04-11, 2026-04-20
    数据库改造          :         db,    2026-04-11, 2026-04-15

    section 测试阶段
    单元测试           :         ut,    2026-04-18, 2026-04-24
    集成测试           :         it,    2026-04-22, 2026-04-28
    性能测试           :         pt,    2026-04-25, 2026-04-29

    section 上线
    预发布环境验证      :         pre,   2026-04-29, 2026-04-30
    灰度发布           :         gray,  2026-05-01, 2026-05-02
    全量上线           :         prod,  2026-05-03, 2026-05-03
```

**里程碑：**
- 📋 **设计冻结**: 2026-04-10
- 🏗️ **功能开发完成**: 2026-04-22
- 🧪 **测试通过**: 2026-04-29
- 🚀 **正式上线**: 2026-05-03

---

## 3. 流程图 - 软件开发流程

```mermaid
flowchart TD
    A[💡 需求分析] --> B[📐 系统设计]
    B --> C[⌨️ 代码实现]
    C --> D[🧪 单元测试]
    D --> E{测试通过?}
    E -->|是| F[🚀 集成测试]
    E -->|否| C
    F --> G{性能达标?}
    G -->|是| H[✅ 上线部署]
    G -->|否| I[⚡ 性能优化]
    I --> F
    H --> J[📊 监控维护]
```

**子流程说明：**
| 步骤 | 说明 |
|------|------|
| 需求分析 | 收集用户需求，整理成需求文档 |
| 系统设计 | 设计系统架构、数据库、接口规范 |
| 代码实现 | 按设计文档进行编码 |
| 单元测试 | 保证每个模块的功能正确性 |

---

## 4. ER 图 - 博客系统数据模型

```mermaid
erDiagram
    USER ||--o{ POST : "发布"
    USER ||--o{ COMMENT : "发表"
    USER ||--o{ LIKE : "点赞"
    POST ||--o{ COMMENT : "包含"
    POST ||--o{ LIKE : "被点赞"
    POST }|--|{ TAG : "打标签"
    CATEGORY ||--o{ POST : "属于"

    USER {
        uuid id PK
        string username
        string email UK
        string password_hash
        string avatar
        timestamp created_at
    }

    POST {
        uuid id PK
        uuid author_id FK
        uuid category_id FK
        string title
        text content
        string cover_image
        int view_count
        timestamp published_at
        timestamp created_at
    }

    COMMENT {
        uuid id PK
        uuid post_id FK
        uuid user_id FK
        uuid parent_id FK "nullable"
        text content
        timestamp created_at
    }

    LIKE {
        uuid id PK
        uuid user_id FK
        uuid post_id FK
        timestamp created_at
    }

    TAG {
        uuid id PK
        string name UK
        string slug
    }

    CATEGORY {
        uuid id PK
        string name
        string slug
    }
```

**关系说明：**
| 符号 | 含义 |
|------|------|
| `\|\|--o{` | 一对多 |
| `}\|--|{` | 多对多 |
| `\|\|--\|\|` | 一对一 |
