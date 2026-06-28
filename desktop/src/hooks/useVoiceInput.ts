import { useRef, useState, useCallback } from 'react';
import { Capacitor } from '@capacitor/core';
import { getBaseUrl } from '../store';

/**
 * useVoiceInput — Microphone recording + ASR transcription hook.
 *
 * Two strategies depending on platform:
 *
 * - **Android (Capacitor)**: Uses a native MediaRecorder plugin
 *   (NativeRecorderPlugin.java) to bypass WebView getUserMedia limitations.
 *   The native recorder captures AAC audio → base64 → JS converts to Blob →
 *   upload to /asr/transcribe.
 *
 * - **Desktop/Browser**: Uses standard MediaRecorder + getUserMedia API.
 */

type VoiceInputOptions = {
  onText: (text: string) => void;
};

export function useVoiceInput({ onText }: VoiceInputOptions) {
  const [recording, setRecording] = useState(false);
  const [transcribing, setTranscribing] = useState(false);
  const [error, setError] = useState('');
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);

  // ─── Android Native Recording ───────────────────────────────────────

  const startNativeRecording = async (): Promise<boolean> => {
    const plugins = Capacitor as unknown as Record<string, {
      requestPermission: () => Promise<{ granted: boolean }>;
      startRecording: () => Promise<void>;
      stopRecording: () => Promise<{ base64: string; mimeType: string; duration: number }>;
    }>;
    const rec = plugins.NativeRecorder;
    if (!rec) return false;

    // Request permission
    try {
      const perm = await rec.requestPermission();
      if (!perm.granted) {
        setError('麦克风权限被拒绝');
        return false;
      }
    } catch {
      return false;
    }

    // Start recording
    await rec.startRecording();
    return true;
  };

  const stopNativeRecording = async (): Promise<void> => {
    const plugins = Capacitor as unknown as Record<string, {
      stopRecording: () => Promise<{ base64: string; mimeType: string; duration: number }>;
    }>;
    const rec = plugins.NativeRecorder;
    if (!rec) return;

    const result = await rec.stopRecording();
    // Convert base64 to Blob
    const byteChars = atob(result.base64);
    const byteNumbers = new Array(byteChars.length);
    for (let i = 0; i < byteChars.length; i++) {
      byteNumbers[i] = byteChars.charCodeAt(i);
    }
    const byteArray = new Uint8Array(byteNumbers);
    const blob = new Blob([byteArray], { type: result.mimeType });

    // Upload for transcription
    setTranscribing(true);
    try {
      const formData = new FormData();
      formData.append('file', blob, 'voice.m4a');

      const res = await fetch(`${getBaseUrl()}/asr/transcribe`, {
        method: 'POST',
        body: formData,
      });

      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.error || `Server error ${res.status}`);
      }

      const data = await res.json();
      if (data.text) {
        onText(data.text);
      } else {
        setError('未能识别语音内容');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '语音识别失败');
    } finally {
      setTranscribing(false);
    }
  };

  // ─── Browser Recording (getUserMedia) ───────────────────────────────

  const startBrowserRecording = async (): Promise<void> => {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus')
      ? 'audio/webm;codecs=opus'
      : MediaRecorder.isTypeSupported('audio/webm')
        ? 'audio/webm'
        : '';

    const mr = new MediaRecorder(stream, mimeType ? { mimeType } : undefined);
    chunksRef.current = [];

    mr.ondataavailable = (e) => {
      if (e.data.size > 0) chunksRef.current.push(e.data);
    };

    mr.onstop = async () => {
      stream.getTracks().forEach((t) => t.stop());
      const blob = new Blob(chunksRef.current, { type: mimeType || 'audio/webm' });
      if (blob.size < 100) {
        setError('录音太短，请重试');
        return;
      }

      setTranscribing(true);
      try {
        const formData = new FormData();
        const ext = mimeType.includes('webm') ? 'webm' : 'wav';
        formData.append('file', blob, `voice.${ext}`);

        const res = await fetch(`${getBaseUrl()}/asr/transcribe`, {
          method: 'POST',
          body: formData,
        });

        if (!res.ok) {
          const err = await res.json().catch(() => ({}));
          throw new Error(err.error || `Server error ${res.status}`);
        }

        const data = await res.json();
        if (data.text) {
          onText(data.text);
        } else {
          setError('未能识别语音内容');
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : '语音识别失败');
      } finally {
        setTranscribing(false);
      }
    };

    mr.start();
    mediaRecorderRef.current = mr;
  };

  // ─── Unified toggle ─────────────────────────────────────────────────

  const startRecording = useCallback(async () => {
    setError('');
    try {
      if (Capacitor.isNativePlatform()) {
        // Android: use native recorder
        const started = await startNativeRecording();
        if (!started) {
          // Fallback: try browser path
          await startBrowserRecording();
        }
      } else {
        // Desktop/browser
        await startBrowserRecording();
      }
      setRecording(true);
    } catch (err) {
      if (err instanceof DOMException && err.name === 'NotAllowedError') {
        setError('麦克风权限被拒绝');
      } else {
        setError('无法访问麦克风: ' + (err instanceof Error ? err.message : '未知错误'));
      }
    }
  }, []);

  const stopRecording = useCallback(async () => {
    setRecording(false);
    try {
      if (Capacitor.isNativePlatform() && !mediaRecorderRef.current) {
        // Native recording
        await stopNativeRecording();
      } else if (mediaRecorderRef.current) {
        // Browser recording
        const mr = mediaRecorderRef.current;
        if (mr.state !== 'inactive') {
          mr.stop();
        }
        mediaRecorderRef.current = null;
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '录音停止失败');
    }
  }, []);

  const toggle = useCallback(() => {
    if (recording) {
      void stopRecording();
    } else {
      void startRecording();
    }
  }, [recording, startRecording, stopRecording]);

  return { recording, toggle, transcribing, error, setError };
}
