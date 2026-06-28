import { useRef, useState, useCallback } from 'react';
import { getBaseUrl } from '../store';

/**
 * useVoiceInput — Microphone recording + ASR transcription hook.
 *
 * Records audio via MediaRecorder API, then uploads the blob to the server's
 * /asr/transcribe endpoint which forwards it to SiliconFlow's TeleSpeechASR.
 *
 * Usage:
 *   const { recording, toggle, transcribing, error } = useVoiceInput({
 *     onText: (text) => appendText(text),
 *   });
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

  const startRecording = useCallback(async () => {
    setError('');
    try {
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
        // Stop all tracks to release the microphone
        stream.getTracks().forEach((t) => t.stop());

        const blob = new Blob(chunksRef.current, {
          type: mimeType || 'audio/webm',
        });

        if (blob.size < 100) {
          setError('录音太短，请重试');
          return;
        }

        // Upload to server for transcription
        setTranscribing(true);
        try {
          const formData = new FormData();
          const ext = mimeType.includes('webm')
            ? 'webm'
            : mimeType.includes('ogg')
              ? 'ogg'
              : 'wav';
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
      setRecording(true);
    } catch (err) {
      if (err instanceof DOMException && err.name === 'NotAllowedError') {
        setError('麦克风权限被拒绝');
      } else {
        setError('无法访问麦克风: ' + (err instanceof Error ? err.message : '未知错误'));
      }
    }
  }, [onText]);

  const stopRecording = useCallback(() => {
    const mr = mediaRecorderRef.current;
    if (mr && mr.state !== 'inactive') {
      mr.stop();
    }
    setRecording(false);
  }, []);

  const toggle = useCallback(() => {
    if (recording) {
      stopRecording();
    } else {
      void startRecording();
    }
  }, [recording, startRecording, stopRecording]);

  return { recording, toggle, transcribing, error, setError };
}
