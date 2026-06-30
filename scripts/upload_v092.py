#!/usr/bin/env python3
"""Upload APK to GitHub release v0.9.2."""
import json
import sys
import urllib.request
import urllib.error

REPO = "hwj123hwj/pi-go"
TOKEN = open("/tmp/gh_token.txt").read().strip()

# 1. Create release
release_data = json.dumps({
    "tag_name": "v0.9.2",
    "target_commitish": "main",
    "name": "Pi-Go Mobile v0.9.2 — Button Fix",
    "body": "## 🔧 Fix: 连接按钮无反应\n\n"
            "**Root cause:** `react-native-gesture-handler` 的 native module 已安装但没有正确初始化。"
            "`GestureHandlerRootView` 必须包裹在根组件中才能让触摸事件正常工作。\n\n"
            "**Fix:**\n"
            "- 添加 `GestureHandlerRootView` 包裹根组件\n"
            "- 添加 `SafeAreaProvider` 提供安全区域\n"
            "- 连接逻辑增加 10 秒超时\n\n"
            "### All fixes since v0.9.0\n"
            "- ✅ v0.9.0: Fixed crash on launch (RNCSafeAreaProvider)\n"
            "- ✅ v0.9.1: Fixed HTTP connection blocked (cleartext traffic)\n"
            "- ✅ v0.9.2: Fixed connect button not responding (gesture handler)",
    "draft": False,
    "prerelease": False,
}).encode()

req = urllib.request.Request(
    "https://api.github.com/repos/{}/releases".format(REPO),
    data=release_data,
    headers={
        "Authorization": "token " + TOKEN,
        "Content-Type": "application/json",
        "Accept": "application/vnd.github+json",
    },
    method="POST",
)

release_id = None
try:
    resp = urllib.request.urlopen(req)
    release = json.loads(resp.read())
    release_id = release["id"]
    print("Created release ID={}".format(release_id))
except urllib.error.HTTPError as e:
    body = e.read().decode()
    print("Error {}: {}".format(e.code, body), file=sys.stderr)
    if e.code == 422:
        req2 = urllib.request.Request(
            "https://api.github.com/repos/{}/releases/tags/v0.9.2".format(REPO),
            headers={"Authorization": "token " + TOKEN, "Accept": "application/vnd.github+json"},
        )
        resp2 = urllib.request.urlopen(req2)
        release = json.loads(resp2.read())
        release_id = release["id"]
        print("Found existing release ID={}".format(release_id))
    else:
        sys.exit(1)

# 2. Upload APK
apk_path = "/home/q/pi-go/mobile/app-release-v0.9.2.apk"
apk_data = open(apk_path, "rb").read()

upload_url = "https://uploads.github.com/repos/{}/releases/{}/assets?name=pigo-rn-release-v0.9.2.apk".format(REPO, release_id)

req3 = urllib.request.Request(
    upload_url,
    data=apk_data,
    headers={
        "Authorization": "token " + TOKEN,
        "Content-Type": "application/vnd.android.package-archive",
        "Accept": "application/vnd.github+json",
    },
    method="POST",
)

resp3 = urllib.request.urlopen(req3)
asset = json.loads(resp3.read())
print("Uploaded APK: {}".format(asset["browser_download_url"]))
