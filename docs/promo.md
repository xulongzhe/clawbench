# 手机上也能 AI 编程了——ClawBench，把终端装进口袋

![Hero](../.clawbench/generated/img_promo_hero_001.jpg)

AI 编程真上瘾啊。

自从用上 Claude Code、CodeBuddy 这些 AI 编程工具，写代码的方式彻底变了——你只管说想法，AI 来写代码，配合得像老搭档一样默契。

但问题是，这种快乐被焊死在了电脑前。

出门遛弯、通勤地铁、躺沙发上刷手机的时候，脑子里突然冒出一个好点子，或者线上出了紧急 Bug 需要快速排查——你只能干瞪眼，因为终端在电脑上跑着呢，够不着。

有没有办法让 AI 编程随时随地都能进行？

有的。用 **ClawBench**。

---

## 一、ClawBench 是什么

ClawBench 是一个开源的移动端 AI 编程工作站，让你可以在手机浏览器上直接使用 Claude Code、CodeBuddy、OpenCode 等 AI 编程工具的**全部能力**。

简单说，它不是又一个聊天套壳——它是 AI 编程工具的**移动化身**。

![Terminal to Mobile](../.clawbench/generated/img_promo_terminal2mobile_001.jpg)

核心能力：

- **原生能力透传**：工具调用、深度思考、Skill 工作流、MCP 插件——零适配成本，编程智能体能干的事，手机上全能干
- **文件全操作**：浏览、编辑、搜索、上传，80+ 文件类型支持，代码语法高亮，行级 Git Diff
- **多智能体调度**：Assistant、Coder、Handyman……用 YAML 配置智能体，不同任务匹配不同角色，还能设定时任务让 AI 定期执行
- **多后端切换**：CodeBuddy、Claude Code、OpenCode 无缝切换，一个平台多种 AI 引擎
- **完全免费 + 开源**：MIT 协议，代码公开，不收一分钱

---

## 二、不是套壳聊天，是真正的编程能力

市面上已经有很多手机端的 AI 聊天工具了，但它们大多只是包了一层对话界面，聊完就完了。

ClawBench 不一样。

它把 AI 编程智能体的**原生能力**完整搬到了手机上——读取文件、编辑代码、执行命令、调用工具……你在电脑终端能做的事，手机上一样不少。

![Features](../.clawbench/generated/img_promo_features_001.jpg)

来点具体的：

**📂 文件浏览**——递归目录浏览，搜索、过滤、排序，重命名、删除、复制、移动，一个长按菜单全搞定

**🎨 代码预览**——highlight.js 语法高亮，行号吸顶，双击左右边缘在文件间跳转

**📝 Markdown 渲染**——渲染/源码一键切换，智能 TOC 目录，LaTeX 数学公式、Mermaid 图表自动渲染

**🤖 AI 智能体**——SSE 流式输出，实时展示 AI 思考过程和工具调用，多会话管理，图片上传多模态对话

**🖼️ 媒体预览**——图片、PDF、音频、视频全支持，灯箱模式缩放拖拽，不用跳转外部 App

**📂 Git 集成**——项目级/文件级提交历史，字符级 Diff 高亮，工作区变更一目了然

---

## 三、实际长什么样

**登录页**——密码保护，安全又精致：

![登录页](screenshots/screenshot-12.jpg)

**AI 对话**——流式输出，工具调用可视化，让 AI 直接读文件、改代码：

![AI对话](screenshots/screenshot-6.jpg)

**代码编辑**——语法高亮，长按弹出编辑菜单，移动端也能改代码：

![代码编辑](screenshots/screenshot-11.jpg)

**Git Diff**——红删绿增，字符级高亮，代码变更清清楚楚：

![Git Diff](screenshots/screenshot-1.jpg)

**Markdown 渲染**——LaTeX 公式、Mermaid 图表，文档阅读体验拉满：

![LaTeX](screenshots/screenshot-5.jpg)

![Mermaid](screenshots/screenshot-8.jpg)

**媒体预览**——图片灯箱、视频播放、PDF 查看，不用跳 App：

![图片预览](screenshots/screenshot-14.jpg)

![视频播放](screenshots/screenshot-15.jpg)

**会话管理**——多会话切换，定时任务调度，AI 也能帮你"定时干活"：

![会话管理](screenshots/screenshot-9.jpg)

---

## 四、架构一图看懂

![Architecture](../.clawbench/generated/img_promo_arch_001.jpg)

ClawBench 的架构非常清晰：

- **手机浏览器 / PWA** 作为前端，Vue 3 + TypeScript 打造移动优先的响应式 UI
- **Go 后端** 提供 HTTP API + SSE 流式推送，连接各个 AI CLI 工具
- **SQLite** 存储会话和配置，轻量可靠
- **AI CLI**（CodeBuddy / Claude Code / OpenCode）作为智能体后端，所有原生能力透明透传

不是简单的 API 转发——是真正把 AI 编程智能体运行在服务器上，手机作为操控终端远程协作。

---

## 五、适合什么场景

![Scene](../.clawbench/generated/img_promo_scene_001.jpg)

- **🚇 通勤路上**：地铁里看一眼 AI 跑到哪了，有问题直接手机上给指令
- **🔥 线上救火**：生产环境出 Bug，掏出手机看日志、让 AI 分析定位，不用等回到工位
- **🛋️ 躺平时刻**：沙发上刷着手机，突然想到一个优化方案，直接让 AI 开工
- **📋 代码审查**：等人等饭的间隙，手机上翻翻最近的提交记录，看看 Diff
- **⏰ 定时任务**：让 AI 每天定时跑报告、检查配置，你只看结果

---

## 六、怎么安装

### 下载

直接去 [GitHub Releases](https://github.com/xulongzhe/clawbench/releases) 下载对应平台的预编译二进制文件，一个文件搞定，无需安装依赖。

```bash
# 1. 下载并解压
wget https://github.com/xulongzhe/clawbench/releases/latest/download/clawbench-linux-amd64.zip
unzip clawbench-linux-amd64.zip

# 2. 配置文件
cd clawbench
cp config.example.yaml config.yaml
# 编辑 config.yaml，至少配置 watch_dir 和 password

# 3. 启动服务
./clawbench-linux-amd64
```

### 手机访问

在手机浏览器输入 `http://你的IP:20000`，就能看到完整界面了。

> 💡 推荐"添加到主屏幕"，PWA 模式体验更流畅，像原生 App 一样运行

### 前提条件

手机上要操控 AI 智能体，电脑（或服务器）上需要先装好对应的 CLI 工具：

- **CodeBuddy**：`npm i -g @anthropic-ai/codebuddy`
- **Claude Code**：`npm i -g @anthropic-ai/claude-code`
- **OpenCode**：`npm i -g opencode`

---

## 七、为什么做 ClawBench

说实话，一开始就是因为自己有这个需求。

用 AI 编程工具写代码真的很爽，但被绑在电脑前很不爽。我想要的是——**随时随地，想用就用**。

市面上的移动端 AI 工具要么只能聊天，要么功能阉割，没有一个能把 AI 编程智能体的完整能力搬上手机。所以我自己做了一个。

ClawBench 的核心理念就一句话：

**不是套壳聊天，是复用编程智能体的全部能力。**

工具调用、深度思考、Skill 工作流、MCP 插件——这些才是 AI 编程的精髓，不该被屏幕大小限制。

---

## 结尾

AI 编程的精髓就是：**你负责想，AI 负责干**。

ClawBench 把这件事从"必须坐在电脑前"变成了"随时随地"。

对了，你现在看到的这篇推文里的配图，就是用 ClawBench 的 AI 智能体 + MiniMax 图像生成能力做的。

试试看吧。

> 🦞 **ClawBench** — 从终端到掌心
>
> GitHub: [github.com/xulongzhe/clawbench](https://github.com/xulongzhe/clawbench)
>
> MIT 协议 · 完全开源 · 免费使用
