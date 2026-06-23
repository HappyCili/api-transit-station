# 图片生成功能实施计划

## 背景与约束

- 复用当前登录模块与现有 JWT 登录态，不重写登录、注册、鉴权 Store 或路由守卫。
- 生图请求逻辑遵循 `gpt-image-2-api(1).md`：
  - 文生图：`POST /v1/images/generations`
  - 图生图：`POST /v1/images/edits`
  - 上传参考图：`POST /v1/uploads`
  - 查询模型：`GET /v1/models`
- 生图 API 鉴权使用用户 API Key：`Authorization: Bearer <api_key>`，不使用网页登录 JWT 作为生图 API Key。
- 页面入口放在普通用户登录后的左侧标签栏中，同时管理员的个人菜单也复用该入口。
- 创作历史与收藏需要服务端持久化。

## 任务清单

### 1. 后端数据与接口

- 新增迁移 `143_user_image_generations.sql`，创建 `user_image_generations` 表。
- 新增服务层模型与仓储接口，保存创作历史、查询分页历史、切换收藏、删除记录。
- 新增 SQL 仓储实现，使用原生 SQL，避免引入 Ent 代码生成范围。
- 新增用户图片历史 Handler：
  - `GET /api/v1/user/image-generations`
  - `POST /api/v1/user/image-generations`
  - `PATCH /api/v1/user/image-generations/:id/favorite`
  - `DELETE /api/v1/user/image-generations/:id`
- 将新 Handler 接入 `Handlers` 聚合、Wire ProviderSet、`wire_gen.go` 手工装配和用户路由。
- 新增或补齐 `/v1/uploads` OpenAI 兼容上传接口；若已存在能力则复用现有实现。

### 2. 前端 API 封装

- 新增 `frontend/src/api/imageGeneration.ts`。
- 使用独立 OpenAI 兼容 Axios client 调用 `/v1/*`，不复用默认 `/api/v1` baseURL。
- 封装：
  - `listImageModels(apiKey)`
  - `uploadReferenceImage(apiKey, file)`
  - `generateImage(apiKey, payload)`
  - `editImage(apiKey, formData)`
  - `listHistory(params)`
  - `saveHistory(record)`
  - `toggleFavorite(id, favorite)`
  - `deleteHistory(id)`
- 错误解析兼容 OpenAI 风格 `{ error: { message, type, code } }`。

### 3. 前端页面与导航

- 新增 `frontend/src/views/user/ImageGenerationView.vue`。
- 页面功能复制当前浏览器参考页的主要功能：
  - 新对话
  - 提示词示例
  - Prompt 输入
  - 上传参考图
  - 提示词优化按钮
  - 模型选择
  - 风格选择
  - 比例选择
  - 画质选择
  - 输出格式选择
  - 数量选择
  - 透明背景与压缩质量
  - 结果预览、下载、复制
  - 创作历史与收藏合集
- 新增 `/image-generation` 路由，要求登录。
- 在 `AppSidebar.vue` 的普通用户菜单和管理员个人菜单新增“图片生成”入口。
- 页面无可用 API Key 时，引导用户到现有 API Key 页面，不自动创建密钥。

### 4. i18n 与类型

- 补充 `frontend/src/i18n/locales/zh.ts` 和 `en.ts` 文案。
- 补充前端类型定义：
  - 请求参数
  - 上传响应
  - OpenAI 图片响应
  - 历史记录 DTO
- 保持文案与现有 Sub2API 风格一致。

### 5. 测试与验证

- 后端测试：
  - 历史接口必须使用 JWT 登录态。
  - 用户只能访问自己的历史记录。
  - 收藏和删除不能操作其他用户记录。
  - 保存历史 payload 校验。
  - `/v1/uploads` 鉴权和文件限制。
- 前端测试：
  - 路由守卫。
  - 侧栏入口渲染。
  - API client 端点、Header、payload 映射。
  - 页面空状态、生成中、成功、失败、收藏切换。
- 运行验证：
  - Go 相关测试。
  - 前端 Vitest 相关测试。
  - TypeScript 构建检查。

## 实施顺序

1. 完成后端历史表、仓储、服务、Handler 和路由。
2. 补齐 `/v1/uploads`。
3. 完成前端 API client 与类型。
4. 完成页面和侧栏/路由接入。
5. 补充 i18n。
6. 添加测试。
7. 运行验证并修复问题。

## 默认决策

- 首版不自动创建 API Key。
- 图片生成结果由前端调用 `/v1/images/generations` 或 `/v1/images/edits` 后，再调用用户历史接口保存。
- 历史保存失败不影响当前生成结果展示。
- 参考图默认先走 `/v1/uploads`，再作为 `reference_images` 传给 `/v1/images/generations`。
