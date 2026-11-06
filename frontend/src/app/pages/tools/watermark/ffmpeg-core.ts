// The FFmpeg.wasm single-thread core, fetched from jsDelivr on the first video
// export and checked against pinned SHA-384 hashes before it is run. A CDN or
// package takeover then cannot run code on this site: a file that differs by
// one byte is refused. Both hashes were taken from the CDN files and matched
// against the files inside the npm tarball of @ffmpeg/core-st@0.11.1 (whose
// sha512 integrity is the one the npm registry publishes).
//
// To upgrade the core: change the version in the URLs, fetch both files, and
// replace the hashes with `openssl dgst -sha384 -binary FILE | openssl base64 -A`.
const BASE = 'https://cdn.jsdelivr.net/npm/@ffmpeg/core-st@0.11.1/dist/';

export const FF_CORE_JS_URL = BASE + 'ffmpeg-core.js';
export const FF_CORE_WASM_URL = BASE + 'ffmpeg-core.wasm';
export const FF_CORE_JS_SHA384 = 'BeUhGK5N9KTyO9NFKUSUBTxwvxLI4EZFbynF9mLtj73/2DqmPqGLk6EJ9ZU+R+6J';
export const FF_CORE_WASM_SHA384 = 'vAbQ21XJMfp9RyAaryzB4ciuXISd636UpiZYr7vDBAuX+NmAYvnQNdvA3RubvdHH';

/** Thrown when a downloaded core file does not match its pinned hash. */
export class CoreIntegrityError extends Error {
  constructor(readonly url: string) {
    super(`Downloaded file failed its integrity check: ${url}`);
    this.name = 'CoreIntegrityError';
  }
}

/** Base64 SHA-384 of the bytes. */
export async function sha384Base64(bytes: ArrayBuffer): Promise<string> {
  if (!globalThis.crypto?.subtle) {
    throw new Error('This browser cannot verify the video engine, so it will not be loaded.');
  }
  const digest = new Uint8Array(await globalThis.crypto.subtle.digest('SHA-384', bytes));
  let binary = '';
  for (const b of digest) binary += String.fromCharCode(b);
  return btoa(binary);
}

export async function fetchVerified(url: string, expected: string, type: string): Promise<string> {
  const response = await fetch(url);
  if (!response.ok) throw new Error(`Could not download the video engine (${response.status}).`);
  const bytes = await response.arrayBuffer();
  if ((await sha384Base64(bytes)) !== expected) throw new CoreIntegrityError(url);
  return URL.createObjectURL(new Blob([bytes], { type }));
}

/**
 * Downloads and verifies the core, returning blob URLs for ffmpeg.js. The
 * worker path is pointed at the (verified) core script because the
 * single-thread core never starts a worker; this keeps ffmpeg.js from
 * fetching a third file from the CDN that nothing has checked.
 */
export async function loadVerifiedCore(): Promise<{ corePath: string; wasmPath: string; workerPath: string }> {
  const [corePath, wasmPath] = await Promise.all([
    fetchVerified(FF_CORE_JS_URL, FF_CORE_JS_SHA384, 'application/javascript'),
    fetchVerified(FF_CORE_WASM_URL, FF_CORE_WASM_SHA384, 'application/wasm'),
  ]);
  return { corePath, wasmPath, workerPath: corePath };
}
