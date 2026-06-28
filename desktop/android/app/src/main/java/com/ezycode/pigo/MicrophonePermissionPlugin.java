package com.ezycode.pigo;

import android.Manifest;
import android.webkit.PermissionRequest;

import com.getcapacitor.JSObject;
import com.getcapacitor.PermissionState;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;
import com.getcapacitor.annotation.Permission;
import com.getcapacitor.annotation.PermissionCallback;

@CapacitorPlugin(
    name = "MicrophonePermission",
    permissions = {
        @Permission(strings = { Manifest.permission.RECORD_AUDIO }, alias = MicrophonePermissionPlugin.ALIAS)
    }
)
public class MicrophonePermissionPlugin extends Plugin {

    static final String ALIAS = "microphone";

    @PluginMethod
    public void request(PluginCall call) {
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
}
