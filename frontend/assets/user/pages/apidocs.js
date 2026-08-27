// API 对接说明
import { esc } from '../api.js';
import { injectIcons } from '../ui.js';
const MD = window.MD;

const APIDOCS = [
  { icon: 'bolt', title: '一、快速开始', desc: '平台提供与 OpenAI 完全兼容的接口，仅需两步：创建 API Key、替换 Base URL。无需修改现有代码即可接入。', blocks: [
    { title: '接口地址', code: 'Base URL: https://mass.yiziyun.com/v1' },
    { title: '身份认证', code: '所有请求需携带请求头 X-API-Key: sk-xxx\n密钥在控制台「API Keys」页创建，请妥善保管，完整密钥仅创建时展示一次。' }
  ] },
  { icon: 'doc', title: '二、发起对话（curl）', desc: '最基础的对话补全请求，一次返回完整结果。', blocks: [
    { title: '非流式请求', code: 'curl https://mass.yiziyun.com/v1/chat/completions \\\n  -H "Content-Type: application/json" \\\n  -H "X-API-Key: sk-你的密钥" \\\n  -d \'{\n    "model": "gpt-4o",\n    "messages": [\n      {"role": "system", "content": "你是一个乐于助人的助手"},\n      {"role": "user", "content": "用一句话介绍你自己"}\n    ]\n  }\'' },
    { title: '流式请求（SSE）', desc: '流式输出打字机效果，适合对话产品。', code: 'curl -N https://mass.yiziyun.com/v1/chat/completions \\\n  -H "Content-Type: application/json" \\\n  -H "X-API-Key: sk-你的密钥" \\\n  -d \'{\n    "model": "gpt-4o",\n    "stream": true,\n    "messages": [{"role": "user", "content": "写一首关于春天的诗"}]\n  }\'' }
  ] },
  { icon: 'code', title: '三、Python SDK', desc: '使用官方 openai 库，只需指定 base_url 与 api_key 即可。', blocks: [
    { title: '安装', code: 'pip install openai' },
    { title: '对话示例', code: 'from openai import OpenAI\n\nclient = OpenAI(\n    base_url="https://mass.yiziyun.com/v1",\n    api_key="sk-你的密钥",\n)\n\nresp = client.chat.completions.create(\n    model="gpt-4o",\n    messages=[{"role": "user", "content": "你好，介绍一下你自己"}],\n)\nprint(resp.choices[0].message.content)' }
  ] },
  { icon: 'code', title: '四、Node.js SDK', desc: '使用官方 openai npm 包，配置 baseURL 即可，支持异步。', blocks: [
    { title: '安装', code: 'npm install openai' },
    { title: '对话示例', code: 'import OpenAI from "openai";\n\nconst client = new OpenAI({\n  baseURL: "https://mass.yiziyun.com/v1",\n  apiKey: "sk-你的密钥",\n});\n\nconst resp = await client.chat.completions.create({\n  model: "gpt-4o",\n  messages: [{ role: "user", content: "你好" }],\n});\nconsole.log(resp.choices[0].message.content);' }
  ] },
  { icon: 'list', title: '五、查询模型列表', desc: '获取平台当前可调用的全部模型及其定价（单价为每 100 万 tokens，人民币）。', blocks: [
    { title: '请求', code: 'curl https://mass.yiziyun.com/v1/models \\\n  -H "X-API-Key: sk-你的密钥"' },
    { title: '返回示例', code: '{"data": [\n  {"id": "kimi-k3", "provider": "openai", "context": "-", "status": "available", "input_price": "¥16", "output_price": "¥80", "cache_read_price": "¥1.6", "cache_write_price": "¥20"}\n]}\n仅展示后台已配置价格的模型，价格与后台「模型价格」实时同步（单价为每 100 万 tokens，人民币；缓存读默认=输入×10%，缓存写默认=输入×125%，后台单独配置后以配置值为准）。' }
  ] },
  { icon: 'coin', title: '六、计费说明', desc: '平台按实际消耗 tokens 计费，输入与输出分别计价，价格以模型广场/模型列表为准。', blocks: [
    { title: '计费方式', code: '费用 = 输入 tokens × 输入单价 + 输出 tokens × 输出单价\n按请求实时扣除账户余额；余额不足时若已开通 Token 授信，将自动使用授信额度垫付。' },
    { title: '费用查看', code: '控制台「用量账单」查看每笔请求消耗与费用，可按日/模型筛选；「交易记录」查看充值、消费流水，可申请开具发票。' }
  ] },
  { icon: 'shield', title: '七、常见错误与处理', desc: '接口错误统一返回 OpenAI 风格 JSON，可按 code 快速定位问题。', blocks: [
    { title: '错误码对照', code: '401 鉴权失败：X-API-Key 缺失或密钥无效，请检查密钥\n403 账号被禁用：请联系管理员\n402 余额不足：请充值或归还授信额度\n404 模型不存在：检查 model 参数拼写\n429 请求过于频繁：触发限流，请降低并发或稍后重试\n500/502/503 上游服务异常：请稍后重试，仍失败请联系管理员' }
  ] }
];

export function initApiDocs() {
  renderApiDocs();
}

function renderApiDocs() {
  const box = document.getElementById('apidoc-list');
  if (!box) return;
  box.innerHTML = APIDOCS.map((doc) => {
    const blocks = doc.blocks.map((b, i) => {
      return '<div class="apidoc-block">' +
        '<div class="apidoc-block__head"><span class="apidoc-block__name">' + esc(b.title) + '</span>' +
        (b.desc ? '<span class="apidoc-block__desc">' + esc(b.desc) + '</span>' : '') +
        '<button class="btn-ghost btn-sm" onclick="copyApiCode(' + i + ')">' + MD.icon('copy', 13) + '复制代码</button></div>' +
        '<pre class="apidoc-code"><code>' + MD.escapeHtml(b.code) + '</code></pre></div>';
    }).join('');
    return '<div class="apidoc-item">' +
      '<div class="apidoc-item__head"><span class="apidoc-item__icon"><span data-ic="' + doc.icon + '" data-sz="16"></span></span>' +
      '<h3 class="apidoc-item__title">' + esc(doc.title) + '</h3></div>' +
      '<p class="apidoc-item__desc">' + esc(doc.desc) + '</p>' +
      blocks + '</div>';
  }).join('');
  injectIcons();
}

export function copyApiCode(i) {
  const box = document.getElementById('apidoc-list');
  const blocks = box.querySelectorAll('.apidoc-code');
  if (blocks[i]) MD.copyText(blocks[i].innerText);
}
window.copyApiCode = copyApiCode;

import { onUserReady } from '../main.js';
onUserReady(initApiDocs);
