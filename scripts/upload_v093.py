#!/usr/bin/env python3
"""Upload APK to GitHub release v0.9.3."""
import json
import sys
import urllib.request
import urllib.error

REPO = "hwj123hwj/pi-go"
TOKEN = open("/tmp/gh_token.txt").read().strip()

release_data = json.dumps({
    "tag_name": "v0.9.3",
    "target_commitish": "main",
    "name": "Pi-Go Mobile v0.9.3 — Connection Hang Fix",
    "body": "## 🔧 Fix: 连接转圈后卡住\n\n"
            "**Root cause:** `/models` API 返回的是 `{ models: [...], current: {...} }` 字典，"
            "但客户端代码把它当数组处理，调 `.map()` 时抛异常。"
            "这个异常虽然被 try/catch 吞掉了，但导致 `init()` 流程中断，"
            "`navigation.replace('List')` 永远不会执行。\n\n"
            "**Fix:**\n"
            "- 正确解析 `/models` 返回的字典结构\n"
            "- 所有 REST 请求加 15 秒超时，防止永久挂起\n"
            "- ASR 上传加 30 秒超时\n\n"
            "### All fixes since v0.9.0\n"
            "- ✅ v0.9.0: Fixed crash on launch (RNCSafeAreaProvider)\n"
            "- ✅ v0.9.1: Fixed HTTP blocked (cleartext traffic)\n"
            "- ✅ v0.9.2: Fixed button not responding (gesture handler)\n"
            "- ✅ v0.9.3: Fixed connection hang (/models API response shape + timeout)",
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
            "https://api.github.com/repos/{}/releases/tags/v0.9.3".format(REPO),
            headers={"Authorization": "token " + TOKEN, "Accept": "application/vnd.github+json"},
        )
        resp2 = urllib.request.urlopen(req2)
        release = json.loads(resp2.read())
        release_id = release["id"]
        print("Found existing release ID={}".format(release_id))
    else:
        sys.exit(1)

apk_path = "/home/q/pi-go/mobile/app-release-v0.9.3.apk"
apk_data = open(apk_path, "rb").read()

upload_url = "https://uploads.github.com/repos/{}/releases/{}/assets?name=pigo-rn-release-v0.9.3.apk".format(REPO, release_id)

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
