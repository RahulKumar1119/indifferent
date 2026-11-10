/**
 * Duration, in seconds, of an audio or video file the user picked, read with
 * an off-screen media element that is never added to the page.
 *
 * Rejects when the browser cannot read the file.
 *
 * The element's src is a `blob:` URL that this document's own
 * URL.createObjectURL just minted for the file. It can never be a
 * `javascript:` or `data:` URL, or carry markup, so nothing the user supplies
 * is interpreted as HTML or script. The scheme is checked right before the
 * assignment to keep it that way (code scanning alerts #4, #6 and #11 were all
 * this one pattern, flagged again each time the copies moved).
 */
export function probeMediaDuration(file: Blob, kind: 'audio' | 'video'): Promise<number> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file);
    const media = document.createElement(kind);
    media.preload = 'metadata';
    media.onloadedmetadata = () => {
      const duration = media.duration;
      URL.revokeObjectURL(url);
      resolve(duration);
    };
    media.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error('This media file could not be read.'));
    };
    if (url.startsWith('blob:')) {
      media.src = url;
    } else {
      URL.revokeObjectURL(url);
      reject(new Error('Unexpected object URL scheme'));
    }
  });
}
