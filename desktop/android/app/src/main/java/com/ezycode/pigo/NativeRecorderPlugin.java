package com.ezycode.pigo;

import android.Manifest;
import android.content.Context;
import android.media.MediaRecorder;
import android.os.Build;
import android.os.SystemClock;
import android.util.Log;

import com.getcapacitor.JSObject;
import com.getcapacitor.PermissionState;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;
import com.getcapacitor.annotation.Permission;
import com.getcapacitor.annotation.PermissionCallback;

import java.io.File;
import java.io.FileInputStream;
import java.io.IOException;

@CapacitorPlugin(
    name = "NativeRecorder",
    permissions = {
        @Permission(strings = { Manifest.permission.RECORD_AUDIO }, alias = NativeRecorderPlugin.ALIAS)
    }
)
public class NativeRecorderPlugin extends Plugin {

    static final String ALIAS = "microphone";
    private static final String TAG = "NativeRecorder";
    private MediaRecorder recorder = null;
    private File outputFile = null;
    private long startTime = 0;

    // ─── Permission ──────────────────────────────────────────────────
    // NOTE: Method names are intentionally NOT "requestPermission"/"checkPermission"
    // because those collide with Capacitor's base Plugin class.

    @PluginMethod
    public void hasPermission(PluginCall call) {
        JSObject ret = new JSObject();
        ret.put("granted", getPermissionState(ALIAS) == PermissionState.GRANTED);
        call.resolve(ret);
    }

    @PluginMethod
    public void askPermission(PluginCall call) {
        if (getPermissionState(ALIAS) == PermissionState.GRANTED) {
            JSObject ret = new JSObject();
            ret.put("granted", true);
            call.resolve(ret);
        } else {
            requestPermissionForAlias(ALIAS, call, "onPermissionResult");
        }
    }

    @PermissionCallback
    private void onPermissionResult(PluginCall call) {
        JSObject ret = new JSObject();
        ret.put("granted", getPermissionState(ALIAS) == PermissionState.GRANTED);
        call.resolve(ret);
    }

    // ─── Recording ───────────────────────────────────────────────────

    @PluginMethod
    public void start(PluginCall call) {
        Log.i(TAG, "start() called");

        // Check permission
        if (getPermissionState(ALIAS) != PermissionState.GRANTED) {
            Log.e(TAG, "No RECORD_AUDIO permission");
            call.reject("PERMISSION_DENIED");
            return;
        }

        try {
            Context ctx = getContext();
            File dir = new File(ctx.getCacheDir(), "recordings");
            if (!dir.exists()) dir.mkdirs();
            outputFile = new File(dir, "voice_" + System.currentTimeMillis() + ".m4a");

            // Release any existing recorder
            if (recorder != null) {
                cleanupRecorder();
            }

            recorder = new MediaRecorder(ctx);
            recorder.setAudioSource(MediaRecorder.AudioSource.MIC);
            recorder.setOutputFormat(MediaRecorder.OutputFormat.MPEG_4);
            recorder.setAudioEncoder(MediaRecorder.AudioEncoder.AAC);
            recorder.setAudioSamplingRate(44100);
            recorder.setAudioEncodingBitRate(128000);
            recorder.setOutputFile(outputFile.getAbsolutePath());

            recorder.prepare();
            recorder.start();
            startTime = SystemClock.elapsedRealtime();

            Log.i(TAG, "Recording started → " + outputFile.getAbsolutePath());
            call.resolve();
        } catch (Exception e) {
            Log.e(TAG, "start() failed", e);
            cleanupRecorder();
            call.reject("START_FAILED: " + e.getMessage());
        }
    }

    @PluginMethod
    public void stop(PluginCall call) {
        Log.i(TAG, "stop() called");

        if (recorder == null) {
            call.reject("NOT_RECORDING");
            return;
        }

        long duration = SystemClock.elapsedRealtime() - startTime;

        try {
            recorder.stop();
            recorder.release();
            recorder = null;
            Log.i(TAG, "Recording stopped. Duration: " + duration + "ms");
        } catch (Exception e) {
            Log.e(TAG, "stop() failed", e);
            cleanupRecorder();
            call.reject("STOP_FAILED: " + e.getMessage());
            return;
        }

        if (outputFile == null || !outputFile.exists()) {
            call.reject("FILE_NOT_FOUND");
            return;
        }

        if (duration < 300) {
            outputFile.delete();
            outputFile = null;
            call.reject("TOO_SHORT");
            return;
        }

        try {
            byte[] data = readFileToBytes(outputFile);
            String base64;
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                base64 = java.util.Base64.getEncoder().encodeToString(data);
            } else {
                base64 = android.util.Base64.encodeToString(data, android.util.Base64.NO_WRAP);
            }

            JSObject ret = new JSObject();
            ret.put("base64", base64);
            ret.put("mimeType", "audio/mp4");
            ret.put("duration", duration);
            call.resolve(ret);
            Log.i(TAG, "Audio returned: " + data.length + " bytes, base64 len=" + base64.length());
        } catch (Exception e) {
            call.reject("READ_FAILED: " + e.getMessage());
        } finally {
            if (outputFile != null) {
                outputFile.delete();
                outputFile = null;
            }
        }
    }

    // ─── Helpers ─────────────────────────────────────────────────────

    private byte[] readFileToBytes(File file) throws IOException {
        byte[] bytes = new byte[(int) file.length()];
        try (FileInputStream fis = new FileInputStream(file)) {
            fis.read(bytes);
        }
        return bytes;
    }

    private void cleanupRecorder() {
        if (recorder != null) {
            try {
                recorder.release();
            } catch (Exception ignored) {
            }
            recorder = null;
        }
        if (outputFile != null) {
            outputFile.delete();
            outputFile = null;
        }
    }
}
