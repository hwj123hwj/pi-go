package com.ezycode.pigo;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.net.Uri;
import android.os.Build;
import android.util.Log;

import androidx.core.content.FileProvider;

import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;

import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;

@CapacitorPlugin(name = "ApkUpdater")
public class ApkUpdaterPlugin extends Plugin {

    private static final String TAG = "ApkUpdater";

    @PluginMethod
    public void downloadAndInstall(PluginCall call) {
        String url = call.getString("url");
        if (url == null || url.isEmpty()) {
            call.reject("url is required");
            return;
        }

        String fileName = call.getString("fileName", "pi-go-update.apk");

        Activity activity = getActivity();
        Context context = activity.getApplicationContext();

        new Thread(() -> {
            HttpURLConnection conn = null;
            try {
                URL downloadUrl = new URL(url);
                conn = (HttpURLConnection) downloadUrl.openConnection();
                conn.setInstanceFollowRedirects(true);
                conn.connect();

                int responseCode = conn.getResponseCode();
                if (responseCode != HttpURLConnection.HTTP_OK) {
                    call.reject("Download failed: HTTP " + responseCode);
                    return;
                }

                int contentLength = conn.getContentLength();
                File outputFile = new File(context.getExternalCacheDir(), fileName);

                InputStream input = conn.getInputStream();
                FileOutputStream output = new FileOutputStream(outputFile);

                byte[] buffer = new byte[8192];
                int bytesRead;
                long totalRead = 0;

                while ((bytesRead = input.read(buffer)) != -1) {
                    output.write(buffer, 0, bytesRead);
                    totalRead += bytesRead;

                    if (contentLength > 0) {
                        int percent = (int) ((totalRead * 100) / contentLength);
                        JSObject progress = new JSObject();
                        progress.put("percent", percent);
                        notifyListeners("downloadProgress", progress);
                    }
                }

                output.close();
                input.close();

                // Trigger install intent via FileProvider
                Uri apkUri = FileProvider.getUriForFile(context,
                    context.getPackageName() + ".fileprovider", outputFile);

                Intent installIntent = new Intent(Intent.ACTION_VIEW);
                installIntent.setDataAndType(apkUri,
                    "application/vnd.android.package-archive");
                installIntent.setFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
                installIntent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);

                activity.runOnUiThread(() -> {
                    try {
                        activity.startActivity(installIntent);
                        JSObject result = new JSObject();
                        result.put("success", true);
                        result.put("path", outputFile.getAbsolutePath());
                        call.resolve(result);
                    } catch (Exception e) {
                        Log.e(TAG, "Failed to start installer", e);
                        call.reject("Failed to start installer: " + e.getMessage());
                    }
                });

            } catch (Exception e) {
                Log.e(TAG, "Download/install failed", e);
                call.reject("Error: " + e.getMessage());
            } finally {
                if (conn != null) conn.disconnect();
            }
        }).start();
    }

    @PluginMethod
    public void getAppVersion(PluginCall call) {
        try {
            Context context = getActivity().getApplicationContext();
            String version = context.getPackageManager()
                .getPackageInfo(context.getPackageName(), 0).versionName;
            JSObject result = new JSObject();
            result.put("version", version);
            call.resolve(result);
        } catch (Exception e) {
            call.reject("Failed to get version: " + e.getMessage());
        }
    }
}
