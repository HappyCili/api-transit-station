# gpt-image-2 图片生成 API 使用文档

> 面向开发者的接入指南。本平台提供 **OpenAI 完全兼容**的图片生成接口，
> 你现有的 OpenAI SDK / 代码只需替换 `base_url` 和 `api_key` 即可调用 `gpt-image-2`。

---

## 目录

1. [快速开始](#一快速开始)
2. [获取 API Key](#二获取-api-key)
3. [接口地址与鉴权](#三接口地址与鉴权)
4. [文生图：/images/generations](#四文生图-imagesgenerations)
5. [图生图：/images/edits](#五图生图-imagesedits)
6. [上传素材：/uploads](#六上传素材-uploads)
7. [查询模型：/models](#七查询模型-models)
8. [请求参数详解](#八请求参数详解)
9. [尺寸与画质映射表](#九尺寸与画质映射表)
10. [响应格式](#十响应格式)
11. [计费规则](#十一计费规则)
12. [配额与限速](#十二配额与限速)
13. [错误码](#十三错误码)
14. [完整示例（curl / Python / Node.js）](#十四完整示例)
15. [常见问题 FAQ](#十五常见问题-faq)

---

## 一、快速开始

最快 30 秒跑通一张图：

```bash
curl https://koimg.com/v1/images/generations \
  -H "Authorization: Bearer sk-koci_你的密钥" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一只戴着宇航头盔的橘猫，漂浮在星云中，电影级光效",
    "size": "1024x1024",
    "quality": "high"
  }'
```

返回：

```json
{
  "created": 1733558400,
  "data": [
    { "url": "https://koimg.com/storage/api-images/xxxx.webp" }
  ]
}
```

---

## 二、获取 API Key

1. 登录平台 → **控制台 / Dashboard → 设置（Settings）→ API 密钥**。
2. 点击「创建密钥」，填写名称（可选设置每日 / 每月积分配额）。
3. 复制生成的密钥，格式为 **`sk-koci_xxxxxxxx`**。

> ⚠️ **密钥只在创建时完整显示一次**，请立即保存。平台仅存储其哈希，无法再次明文展示。
> 密钥泄露后请立刻在设置页删除并重建。

| 密钥属性 | 说明 |
|---------|------|
| 前缀 | 固定为 `sk-koci_` |
| 归属 | 绑定到你的用户账户，调用产生的积分从你的账户扣除 |
| 每日配额 `daily_limit` | 可选。单密钥当日累计可消耗积分上限，超出返回 429 |
| 每月配额 `monthly_limit` | 可选。单密钥当月累计可消耗积分上限，超出返回 429 |

---

## 三、接口地址与鉴权

| 项 | 值 |
|----|-----|
| **Base URL** | `https://koimg.com/v1` |
| **鉴权方式** | HTTP Header：`Authorization: Bearer sk-koci_<你的密钥>` |
| **Content-Type** | `application/json`（文生图）/ `multipart/form-data`（图生图、上传） |

> 注意：这里用的是**用户 API 密钥**（`sk-koci_*`），**不是**网页登录用的 JWT Token。

> ℹ️ **关于 Base URL**：推荐填标准的 `https://koimg.com/v1`，与 OpenAI 官方
> `https://api.openai.com/v1` 写法一致，任何 OpenAI SDK 都能直接用。
> 平台也兼容 `https://koimg.com/api/v1`（二者等价，前者只是后者的标准别名），
> 但建议统一用 `/v1`。

兼容接口清单（基于 Base URL 拼接）：

| 方法 | 路径 | 用途 |
|------|------|------|
| `POST` | `/images/generations` | 文生图（同步返回） |
| `POST` | `/images/edits` | 图生图 / 参考图编辑（multipart） |
| `POST` | `/uploads` | 上传参考图，返回平台托管 URL |
| `GET`  | `/models` | 列出当前可用模型 |

### 用 OpenAI 官方 SDK 接入

只需把 `base_url` 指向本平台即可，无需改其它代码：

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-koci_你的密钥",
    base_url="https://koimg.com/v1",
)

result = client.images.generate(
    model="gpt-image-2",
    prompt="赛博朋克城市夜景，霓虹倒影，雨后街道",
    size="1536x1024",
    quality="high",
)
print(result.data[0].url)
```

---

## 四、文生图 `/images/generations`

**`POST https://koimg.com/v1/images/generations`**

请求体（JSON）：

```json
{
  "model": "gpt-image-2",
  "prompt": "描述你想要的画面",
  "n": 1,
  "size": "1024x1024",
  "quality": "high",
  "response_format": "url",
  "style": "vivid",
  "background": "auto",
  "output_format": "webp",
  "output_compression": 80,
  "moderation": "auto"
}
```

最少只需 `prompt` 一个字段，其余都有默认值。

---

## 五、图生图 `/images/edits`

基于一张或多张参考图生成新图，使用 **multipart/form-data** 上传图片。

**`POST https://koimg.com/v1/images/edits`**

```bash
curl https://koimg.com/v1/images/edits \
  -H "Authorization: Bearer sk-koci_你的密钥" \
  -F model="gpt-image-2" \
  -F prompt="把这张照片改成水彩画风格" \
  -F size="1024x1024" \
  -F quality="high" \
  -F image=@/path/to/photo.jpg
```

支持的图片字段名（可多张）：

| 字段名 | 说明 |
|--------|------|
| `image` / `image_2` / `image_3` | 本平台标准多图字段 |
| `image[]` | PHP / curl 数组风格 |
| `images` | 批量字段 |
| `mask` | OpenAI 的蒙版字段 —— **会被忽略** |

- 单张图片 **≤ 10MB**，否则返回 400。
- 每张参考图都会经过内容审核，违规返回 400。
- 文本字段（`prompt` / `size` / `quality` / `style` / `n` / `output_format` / `background` / `moderation` / `output_compression`）含义与文生图一致。

---

## 六、上传素材 `/uploads`

如果你想用 **JSON 接口**做图生图（不想用 multipart），可先上传图片拿到平台 URL，
再把 URL 放进 `/images/generations` 的 `reference_images` 字段。

**`POST https://koimg.com/v1/uploads`**（multipart）

```bash
curl https://koimg.com/v1/uploads \
  -H "Authorization: Bearer sk-koci_你的密钥" \
  -F purpose="reference" \
  -F file=@/path/to/photo.jpg
```

返回：

```json
{ "object": "file", "purpose": "reference", "url": "https://koimg.com/storage/references/xxxx.webp", "bytes": 204800 }
```

| `purpose` | 限制 | 用途 |
|-----------|------|------|
| `reference`（默认） | 图片 ≤ 10MB，经内容审核 | 作 `reference_images[]` |
| `input_video` | mp4 ≤ 100MB | 视频接口用（非本文档范围） |

然后在文生图里引用：

```json
{
  "model": "gpt-image-2",
  "prompt": "把参考图变成赛博朋克风格",
  "reference_images": ["https://koimg.com/storage/references/xxxx.webp"]
}
```

> 🔒 **安全限制**：`reference_images` 里的 URL **必须**是本平台上传接口返回的地址。
> 外部 URL 会被拒绝（400），这是为了防止 SSRF 和绕过内容审核。

---

## 七、查询模型 `/models`

**`GET https://koimg.com/v1/models`**

```bash
curl https://koimg.com/v1/models \
  -H "Authorization: Bearer sk-koci_你的密钥"
```

返回当前启用的图片 / 视频模型列表：

```json
{
  "object": "list",
  "data": [
    { "id": "gpt-image-2", "object": "model", "owned_by": "kocodeimg", "kind": "image" }
  ]
}
```

---

## 八、请求参数详解

`/images/generations` 与 `/images/edits` 共用的参数：

| 参数 | 类型 | 默认 | 说明 |
|------|------|------|------|
| `model` | string | `gpt-image-2` | 模型名，见 `/models` |
| `prompt` | string | **必填** | 画面描述，最长 **32000** 字符 |
| `n` | int | `1` | 生成张数。OpenAI 规范允许 1–10，**本平台内部最多裁剪到 4** |
| `size` | string | `1024x1024` | 像素尺寸或比例，详见[映射表](#九尺寸与画质映射表) |
| `quality` | string | 由 `size` 反推 | 画质档位，详见下方说明 |
| `response_format` | string | `url` | `url` 或 `b64_json` |
| `style` | string | `vivid` | `vivid`（鲜艳）/ `natural`（自然） |
| `background` | string | `auto` | `auto` / `opaque`（不透明）/ `transparent`（透明） |
| `output_format` | string | `webp` | `png` / `jpeg` / `webp` |
| `output_compression` | int | 无 | 0–100，仅对 `webp` / `jpeg` 生效 |
| `moderation` | string | 无 | `low` / `auto`，内容审核宽松度 |
| `user` | string | 无 | 调用方自定义标识（仅用于分析，可省略） |
| `reference_images` | string[] | 无 | 参考图 URL 列表（必须来自 `/uploads`），等效图生图 |

### `quality` 取值

平台同时接受 **三套词汇**，最终都会归一化到内部档位 `1k` / `2k` / `4k`：

| 你传的值 | 归一化档位 | 来源规范 |
|----------|-----------|---------|
| `low` | 1k | gpt-image-1 |
| `medium` | 2k | gpt-image-1 |
| `high` | 4k | gpt-image-1 |
| `standard` | 1k | dall-e-3 |
| `hd` | 2k | dall-e-3 |
| `1k` / `2k` / `4k` | 原样 | 本平台 |
| `auto` 或不传 | 由 `size` 像素反推 | —— |

> 不传 `quality` 时，平台按 `size` 的**最长边**反推档位：
> ≥2600px → 4k，≥1700px → 2k，否则 → 1k。
> 因此用标准 OpenAI 客户端只传 `size`（如 `3840x2160`）也能拿到高清图。

---

## 九、尺寸与画质映射表

`gpt-image-2` 支持任意分辨率，但本平台把 `size` 归一化为**比例**，再结合 `quality` 档位
映射到实际输出像素。`size` 只识别以下 5 种比例，**其它一律按 1:1 处理**：

| 传入 `size` | 识别比例 |
|-------------|---------|
| `1024x1024`、`1:1` | 1:1 |
| `1536x1024`、`16:9` | 16:9 |
| `1024x1536`、`9:16` | 9:16 |
| `4:3` | 4:3 |
| `3:4` | 3:4 |
| `auto` 或无法识别 | 1:1 |

实际输出像素（比例 × 画质档位）：

| 比例 | 1k（low/standard） | 2k（medium/hd） | 4k（high） |
|------|--------------------|-----------------|-----------|
| 1:1  | 1024×1024 | 2048×2048 | 2880×2880 |
| 16:9 | 1536×1024 | 2560×1440 | 3840×2160 |
| 9:16 | 1024×1536 | 1440×2560 | 2160×3840 |
| 4:3  | 1536×1024 | 2560×1440 | 3840×2160 |
| 3:4  | 1024×1536 | 1440×2560 | 2160×3840 |

> 💡 想要 16:9 的 4K 大图：`{"size": "16:9", "quality": "high"}` → 输出 3840×2160。

---

## 十、响应格式

成功返回 `200`，OpenAI 标准结构：

```json
{
  "created": 1733558400,
  "data": [
    {
      "url": "https://koimg.com/storage/api-images/xxxx.webp",
      "b64_json": "iVBORw0KGgo...",
      "revised_prompt": null
    }
  ]
}
```

| `response_format` | 返回内容 |
|-------------------|---------|
| `url`（默认） | `data[].url` 平台托管地址 **＋** `data[].b64_json`（同时附带，方便直接用） |
| `b64_json` | 仅 `data[].b64_json`（base64 编码图片，不含 URL） |

- 图片始终会在平台存储一份（用于你的「创作历史」可见）。
- `data` 数组长度 = 实际成功生成的张数。若部分失败，仅返回成功的，失败部分会**退还积分**。

---

## 十一、计费规则

- **按张计费**：每次请求扣除 `单价 × n` 积分。
- 单价由你的**套餐 / 用户专属价格**和**画质档位**共同决定（4k 通常高于 1k）。
- **先扣后退**：请求开始时按 `n` 张全额预扣；若某些图生成失败，对应积分**自动退还**到账户。
- 全部失败时返回 `502`，并已退还全部预扣积分。

> 积分余额不足时返回 **402**，请先充值。

---

## 十二、配额与限速

每个 API Key 可单独设置配额（在设置页编辑）：

| 配额 | 触发 | 返回 |
|------|------|------|
| `daily_limit` | 当日累计消耗积分超限 | `429` + `API Key 每日配额已满` |
| `monthly_limit` | 当月累计消耗积分超限 | `429` + `API Key 月度配额已满` |

- 配额按**积分**计算，不是按请求次数。
- 日配额计数每日 0 点（UTC）重置，月配额每月重置。
- 不设置配额（留空）= 不限制，仅受账户总余额约束。

---

## 十三、错误码

所有错误都返回 OpenAI 风格结构：

```json
{ "error": { "message": "原因描述", "type": "错误类型", "code": "可选代码" } }
```

| HTTP | type | 含义 | 处理建议 |
|------|------|------|---------|
| `401` | `invalid_request_error` | 缺少 / 无效 API Key | 检查 `Authorization` 头与密钥 |
| `400` | `invalid_request_error` | 参数错误 / prompt 含违禁词 / 审核未通过 / 参考图非平台 URL | 按 message 修正 |
| `402` | `insufficient_quota` | 账户积分余额不足 | 充值 |
| `403` | `insufficient_quota` | 当前套餐无权使用该模型 | 升级套餐 |
| `429` | `insufficient_quota` | API Key 配额已满 | 等待重置或调高配额 |
| `502` | `api_error` | 所有图片生成均失败（已退款） | 缩短 prompt / 稍后重试 |
| `422` | `invalid_request_error` | 图生图缺少 `prompt` 或图片文件 | 补齐字段 |

---

## 十四、完整示例

### curl —— 文生图（透明背景 PNG）

```bash
curl https://koimg.com/v1/images/generations \
  -H "Authorization: Bearer sk-koci_你的密钥" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一个极简风格的 logo 图标，扁平设计",
    "size": "1:1",
    "quality": "high",
    "background": "transparent",
    "output_format": "png"
  }'
```

### Python —— OpenAI SDK

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-koci_你的密钥",
    base_url="https://koimg.com/v1",
)

resp = client.images.generate(
    model="gpt-image-2",
    prompt="水墨风格的山水画，远山云雾缭绕",
    size="1536x1024",
    quality="high",
)
print(resp.data[0].url)
```

### Python —— requests（手写请求 + 下载 base64）

```python
import base64
import requests

resp = requests.post(
    "https://koimg.com/v1/images/generations",
    headers={"Authorization": "Bearer sk-koci_你的密钥"},
    json={
        "model": "gpt-image-2",
        "prompt": "未来主义机甲战士，金属质感，工作室打光",
        "size": "9:16",
        "quality": "high",
        "response_format": "b64_json",
    },
    timeout=300,
)
resp.raise_for_status()
b64 = resp.json()["data"][0]["b64_json"]
with open("out.webp", "wb") as f:
    f.write(base64.b64decode(b64))
print("已保存 out.webp")
```

### Node.js —— OpenAI SDK

```javascript
import OpenAI from "openai";

const client = new OpenAI({
  apiKey: "sk-koci_你的密钥",
  baseURL: "https://koimg.com/v1",
});

const result = await client.images.generate({
  model: "gpt-image-2",
  prompt: "一杯冒着热气的咖啡，木桌，晨光，浅景深",
  size: "1024x1024",
  quality: "high",
});

console.log(result.data[0].url);
```

### Python —— 图生图（先上传，再用 JSON 引用）

```python
import requests

BASE = "https://koimg.com/v1"
HEAD = {"Authorization": "Bearer sk-koci_你的密钥"}

# 1) 上传参考图
with open("photo.jpg", "rb") as f:
    up = requests.post(f"{BASE}/uploads",
                       headers=HEAD,
                       data={"purpose": "reference"},
                       files={"file": f}).json()
ref_url = up["url"]

# 2) 用参考图生成
out = requests.post(f"{BASE}/images/generations",
                    headers=HEAD,
                    json={
                        "model": "gpt-image-2",
                        "prompt": "把这张照片转成吉卜力动画风格",
                        "reference_images": [ref_url],
                        "size": "1024x1024",
                        "quality": "high",
                    }).json()
print(out["data"][0]["url"])
```

### curl —— 图生图（直接 multipart 上传）

```bash
curl https://koimg.com/v1/images/edits \
  -H "Authorization: Bearer sk-koci_你的密钥" \
  -F model="gpt-image-2" \
  -F prompt="给这张人像换成赛博朋克霓虹背景" \
  -F size="1024x1024" \
  -F quality="high" \
  -F image=@portrait.jpg
```

---

## 十五、常见问题 FAQ

**Q：为什么我传 `n=10` 只生成了 4 张？**
A：OpenAI 规范允许 1–10，但本平台为防滥用内部裁剪到最多 4 张。

**Q：为什么我指定 `size=800x600`，结果尺寸不是 800×600？**
A：平台先把 `size` 归一化为比例（识别 1:1 / 16:9 / 9:16 / 4:3 / 3:4，其余按 1:1），
再按 `quality` 档位映射到固定输出像素。请参考[映射表](#九尺寸与画质映射表)按比例+档位组合。

**Q：`url` 和 `b64_json` 有什么区别？**
A：`response_format=url` 时**两者都返回**（URL 是平台托管地址，base64 方便直接落盘）；
`b64_json` 时只返回 base64。

**Q：透明背景为什么没生效 / 变成了 PNG？**
A：透明背景需要 alpha 通道，`jpeg` 不支持，平台会自动改用 `png`。
若 `output_format` 传了 `jpeg` 又要 `transparent`，会被强制转 `png`。

**Q：图片会保留多久 / 在哪看？**
A：通过 API 生成的图会存入你的账户「创作历史」，可在后台查看。

**Q：调用超时设多少合适？**
A：高清大图生成较慢，建议客户端超时设 **≥ 120 秒**（4K 图可设 300 秒）。

**Q：报 400 `Prompt contains disallowed content` 怎么办？**
A：提示词命中了违禁词或内容审核。请修改描述，避免敏感 / 违规内容。

**Q：报 502 但积分没扣？**
A：所有图都失败时平台会**自动退还**全部预扣积分，无需担心。请缩短 prompt 后重试。

---

> 文档对应接口：`POST /v1/images/generations`、`/v1/images/edits`、`/v1/uploads`、`/v1/models`
> Base URL：`https://koimg.com/v1` ｜ 鉴权：`Authorization: Bearer sk-koci_*`
> 与 OpenAI Images API 完全兼容，可直接复用官方 SDK。
