## Easy Code Added Memories
- DeepVcodeClient 项目路径: /Users/weijian/codex_work/DeepVcodeClient — 竞品项目，常用于功能参考和对比分析
- 移动端 ASR（语音转文字）功能在 Capacitor WebView 中不工作。尝试了三层修复（OS权限插件、WebChromeClient授权、原生MediaRecorder插件+registerPlugin），但仍显示 "speech-to-text service error"。根因可能是 Capacitor WebView 的音频录制限制。已记录为已知问题，计划通过 React Native 重构解决。
