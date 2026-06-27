import re, json, urllib.request

with open('/home/q/.git-credentials') as f:
    cred_line = f.readline().strip()
m = re.search(r'hwj123hwj:([^@]+)@', cred_line)
token = m.group(1)

REPO = "hwj123hwj/pi-go"
APK_PATH = "/home/q/pi-go/pi-go-debug.apk"
headers = {'Authorization': f'token {token}', 'Accept': 'application/vnd.github+json'}

# Create new release v0.6.0
release_body = """## v0.6.0 — 应用内自更新 + 移动端工具栏精简

### 🔄 应用内自更新
- 检测到新版本时自动弹出更新对话框
- 应用内直接下载 APK 并唤起系统安装器
- 下载进度实时显示
- 无需跳转浏览器

### 🧹 移动端工具栏精简
- 隐藏密度切换器（移动端不需要）
- 隐藏侧边栏和底部终端开关（移动端用不到）
- 只保留右侧面板按钮
- 标题左对齐，按钮右对齐（space-between 布局）

### 🐛 代码审查修复（v33-v36）
- 修复4轮共10个 Bug：CSS类名错误、内存泄漏、触摸事件冲突、遮罩重叠等"""

body = json.dumps({
    "tag_name": "v0.6.0",
    "target_commitish": "main",
    "name": "v0.6.0 — 应用内自更新",
    "body": release_body,
    "draft": False,
    "prerelease": False
}).encode()

req = urllib.request.Request(
    f'https://api.github.com/repos/{REPO}/releases',
    data=body, headers=headers, method='POST')
resp = json.loads(urllib.request.urlopen(req).read())
release_id = resp['id']
print(f"✅ Created release v0.6.0 (id={release_id})")

# Upload APK
with open(APK_PATH, 'rb') as f:
    apk_data = f.read()
req = urllib.request.Request(
    f'https://uploads.github.com/repos/{REPO}/releases/{release_id}/assets?name=pi-go-debug.apk',
    data=apk_data,
    headers={**headers, 'Content-Type': 'application/vnd.android.package-archive'},
    method='POST')
resp = json.loads(urllib.request.urlopen(req).read())
print(f"✅ Uploaded: {resp['name']} ({resp['size']:,} bytes)")
print(f"   {resp['browser_download_url']}")
