#!/usr/bin/env python3
"""Create GitHub release v0.9.1 and upload APK."""
import json
import os
import sys
import urllib.request
import urllib.error

REPO = "hwj123hwj/pi-go"
TOKEN = open("/tmp/gh_token.txt").read().strip()

# 1. Create release
release_data = json.dumps({
    "tag_name": "v0.9.1",
    "target_commitish": "main",
    "name": "Pi-Go Mobile v0.9.1 — Cleartext Traffic Fix",
    "body": "## 🔧 Fix: 无法连接服务器 (Network request failed)\n\n"
            "**Root cause:** Android 9+ 默认禁止所有 HTTP 明文流量。"
            "你的服务器 `http://8.141.97.21:8080` 使用 HTTP，"
            "所以 fetch 请求被系统拦截。\n\n"
            "**Fix:** Added `android:usesCleartextTraffic=true` to allow HTTP connections.\n\n"
            "### What was fixed\n"
            "- ✅ v0.9.0: Fixed crash on launch (RNCSafeAreaProvider)\n"
            "- ✅ v0.9.1: Fixed HTTP connection error (cleartext traffic)",
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
            "https://api.github.com/repos/{}/releases/tags/v0.9.1".format(REPO),
            headers={"Authorization": "token " + TOKEN, "Accept": "application/vnd.github+json"},
        )
        resp2 = urllib.request.urlopen(req2)
        release = json.loads(resp2.read())
        release_id = release["id"]
        print("Found existing release ID={}".format(release_id))
    else:
        sys.exit(1)

# 2. Upload APK
apk_path = "/home/q/pi-go/mobile/app-release-v0.9.1.apk"
apk_data = open(apk_path, "rb").read()

upload_url = "https://uploads.github.com/repos/{}/releases/{}/assets?name=pigo-rn-release-v0.9.1.apk".format(REPO, release_id)

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
