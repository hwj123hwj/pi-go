package com.ezycode.pigo;

import android.os.Bundle;
import android.webkit.PermissionRequest;
import android.webkit.WebChromeClient;
import android.webkit.WebView;

import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {

    @Override
    public void onCreate(Bundle savedInstanceState) {
        registerPlugin(ApkUpdaterPlugin.class);
        registerPlugin(MicrophonePermissionPlugin.class);
        registerPlugin(NativeRecorderPlugin.class);
        super.onCreate(savedInstanceState);

        // Auto-grant WebView permission requests for getUserMedia (fallback path)
        WebView webView = this.bridge.getWebView();
        if (webView != null) {
            webView.setWebChromeClient(new WebChromeClient() {
                @Override
                public void onPermissionRequest(final PermissionRequest request) {
                    request.grant(request.getResources());
                }
            });
        }
    }
}
