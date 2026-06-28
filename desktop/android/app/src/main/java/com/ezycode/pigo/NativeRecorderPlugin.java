package com.ezycode.pigo;

import android.Manifest;
import android.content.Context;
import android.content.pm.PackageManager;
import android.media.MediaRecorder;
import android.os.Build;
import android.os.SystemClock;
import android.util.Log;

import androidx.core.content.ContextCompat;

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
import java.util.Base64;

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

    @PluginMethod
    public void checkPermission(PluginCall call) {
        JSObject ret = new JSObject();
        ret.put("granted", getPermissionState(ALIAS) == PermissionState.GRANTED);
        call.resolve(ret);
    }

    @PluginMethod
    public void requestPermission(PluginCall call) {
        if (getPermissionState(ALIAS) == PermissionState.GRANTED) {
            JSObject ret = new JSObject();
            ret.put("granted", true);
            call.resolve(ret);
        } else {
            requestPermissionForAlias(ALIAS, call, "permissionCallback");
        }
    }

    @PermissionCallback
    private void permissionCallback(PluginCall call) {
        JSObject ret = new JSObject();
        ret.put("granted", getPermissionState(ALIAS) == PermissionState.GRANTED);
        call.resolve(ret);
    }

    @PluginMethod
    public void startRecording(PluginCall call) {
        // Check permission first
        if (getPermissionState(ALIAS) != PermissionState.GRANTED) {
            call.reject("RECORD_AUDIO permission not granted");
            return;
        }

        try {
            Context ctx = getContext();

            // Create output file in cache dir
            File dir = new File(ctx.getCacheDir(), "recordings");
            if (!dir.exists()) dir.mkdirs();
            outputFile = new File(dir, "voice_" + System.currentTimeMillis() + ".m4a");

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

            Log.i(TAG, "Recording started: " + outputFile.getAbsolutePath());
            call.resolve();
        } catch (Exception e) {
            Log.e(TAG, "Failed to start recording", e);
            cleanupRecorder();
            call.reject("Failed to start recording: " + e.getMessage());
        }
    }

    @PluginMethod
    public void stopRecording(PluginCall call) {
        if (recorder == null) {
            call.reject("Not recording");
            return;
        }

        long duration = SystemClock.elapsedRealtime() - startTime;

        try {
            recorder.stop();
            recorder.release();
            recorder = null;
            Log.i(TAG, "Recording stopped. Duration: " + duration + "ms");
        } catch (Exception e) {
            Log.e(TAG, "Failed to stop recording", e);
            cleanupRecorder();
            call.reject("Failed to stop recording: " + e.getMessage());
            return;
        }

        if (outputFile == null || !outputFile.exists()) {
            call.reject("Recording file not found");
            return;
        }

        // Too short?
        if (duration < 300) {
            outputFile.delete();
            call.reject("Recording too short");
            return;
        }

        // Read file and return as base64
        try {
            byte[] data = readFileToBytes(outputFile);
            String base64;
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                base64 = Base64.getEncoder().encodeToString(data);
            } else {
                base64 = android.util.Base64.encodeToString(data, android.util.Base64.NO_WRAP);
            }

            JSObject ret = new JSObject();
            ret.put("base64", base64);
            ret.put("mimeType", "audio/mp4");
            ret.put("duration", duration);
            call.resolve(ret);
        } catch (Exception e) {
            call.reject("Failed to read recording: " + e.getMessage());
        } finally {
            if (outputFile != null) {
                outputFile.delete();
                outputFile = null;
            }
        }
    }

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
