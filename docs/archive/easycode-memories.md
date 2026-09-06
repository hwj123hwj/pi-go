## Easy Code Added Memories
- DeepVcodeClient 项目路径: /Users/weijian/codex_work/DeepVcodeClient — 竞品项目，常用于功能参考和对比分析
- 移动端 ASR（语音转文字）功能在 Capacitor WebView 中不工作。尝试了三层修复（OS权限插件、WebChromeClient授权、原生MediaRecorder插件+registerPlugin），但仍显示 "speech-to-text service error"。根因可能是 Capacitor WebView 的音频录制限制。已记录为已知问题，计划通过 React Native 重构解决。
- 并行开发审核流程标准：每个 agent 开发完成后必须自己跑 go build/vet/test 并全部 PASS，自己 commit，然后报告自检结果（Build/Vet/Tests/Commit hash/文件清单）。我只需要快速 diff review 确认无架构冲突后合并。不要再让 agent 丢未提交的代码给我全量审核。
