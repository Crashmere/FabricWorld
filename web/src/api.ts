import { ref } from "vue";
import type { Photo } from "./types";
export const base = import.meta.env.BASE_URL;
export const notice = ref("");
let noticeTimer: ReturnType<typeof setTimeout>;
export function toast(text: string) {
  notice.value = text;
  clearTimeout(noticeTimer);
  noticeTimer = setTimeout(() => (notice.value = ""), 4500);
}
export class APIError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public field = "",
  ) {
    super(message);
  }
}
export function key() {
  const b = new Uint8Array(16);
  crypto.getRandomValues(b);
  return Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
}
export const mediaURL = (id: string, variant = "thumb") =>
  base + "media/" + id + "/" + variant;
export async function request<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  let response: Response;
  try {
    response = await fetch(base + "api/" + path, {
      ...options,
      signal: options.signal ?? AbortSignal.timeout(55000),
    });
  } catch {
    throw new APIError(
      0,
      "network",
      "连接中断，输入已保留。请检查网络后重试。",
    );
  }
  if (!response.ok) {
    let e: { code?: string; message?: string; field?: string } = {};
    try {
      e = await response.json();
    } catch {}
    throw new APIError(
      response.status,
      e.code ?? "server",
      e.message ?? "服务暂时不可用，请稍后重试",
      e.field,
    );
  }
  return response.json();
}
export async function write<T>(
  path: string,
  method: string,
  body: unknown,
  operationKey: string,
): Promise<T> {
  try {
    return await request<T>(path, {
      method,
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": operationKey,
      },
      body: JSON.stringify(body),
    });
  } catch (e) {
    if (e instanceof APIError && (e.status === 0 || e.status >= 500)) {
      try {
        return await request<T>("operations/" + operationKey);
      } catch {}
    }
    throw e;
  }
}
export function upload(
  file: File,
  operationKey: string,
  onProgress: (n: number) => void,
): Promise<Photo> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", base + "api/uploads");
    xhr.timeout = 120000;
    xhr.setRequestHeader("Idempotency-Key", operationKey);
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable)
        onProgress(Math.min(95, Math.round((e.loaded / e.total) * 95)));
    };
    xhr.onload = () => {
      let data: any;
      try {
        data = JSON.parse(xhr.responseText);
      } catch {}
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(data);
      } else {
        reject(
          new APIError(
            xhr.status,
            data?.code ?? "upload",
            data?.message ?? "照片上传失败，请重试",
          ),
        );
      }
    };
    xhr.onerror = () => reject(new APIError(0, "network", "上传中断，请重试"));
    xhr.ontimeout = () =>
      reject(new APIError(0, "timeout", "上传超时，请重试"));
    const body = new FormData();
    body.append("file", file);
    xhr.send(body);
  });
}
