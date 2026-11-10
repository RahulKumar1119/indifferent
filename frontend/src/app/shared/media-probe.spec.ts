import { probeMediaDuration } from './media-probe';

/** A real, playable mono 8 kHz 16-bit WAV of the given length, built in memory. */
export function wav(seconds: number): Blob {
  const rate = 8000;
  const samples = Math.round(rate * seconds);
  const buf = new ArrayBuffer(44 + samples * 2);
  const v = new DataView(buf);
  const text = (o: number, s: string) => [...s].forEach((c, i) => v.setUint8(o + i, c.charCodeAt(0)));
  text(0, 'RIFF');
  v.setUint32(4, 36 + samples * 2, true);
  text(8, 'WAVE');
  text(12, 'fmt ');
  v.setUint32(16, 16, true);
  v.setUint16(20, 1, true);
  v.setUint16(22, 1, true);
  v.setUint32(24, rate, true);
  v.setUint32(28, rate * 2, true);
  v.setUint16(32, 2, true);
  v.setUint16(34, 16, true);
  text(36, 'data');
  v.setUint32(40, samples * 2, true);
  return new Blob([buf], { type: 'audio/wav' });
}

describe('probeMediaDuration', () => {
  it('reads the duration of a real file without adding anything to the page', async () => {
    const before = document.querySelectorAll('audio, video').length;
    const d = await probeMediaDuration(wav(1.5), 'audio');
    expect(d).toBeCloseTo(1.5, 1);
    expect(document.querySelectorAll('audio, video').length).toBe(before);
  });

  it('rejects a file the browser cannot read', async () => {
    const junk = new Blob([new Uint8Array([1, 2, 3, 4, 5, 6, 7, 8])], { type: 'video/mp4' });
    await expectAsync(probeMediaDuration(junk, 'video')).toBeRejectedWithError('This media file could not be read.');
  });

  it('only ever points the media element at a blob: URL, and releases it', async () => {
    const revoke = spyOn(URL, 'revokeObjectURL').and.callThrough();
    const create = spyOn(URL, 'createObjectURL').and.callThrough();
    await probeMediaDuration(wav(0.5), 'audio');
    const minted = create.calls.mostRecent().returnValue;
    expect(minted.startsWith('blob:')).toBeTrue();
    expect(revoke).toHaveBeenCalledWith(minted);
  });

  it('refuses any URL that is not blob: and never assigns it', async () => {
    spyOn(URL, 'createObjectURL').and.returnValue('javascript:alert(1)');
    const setSrc = spyOnProperty(HTMLMediaElement.prototype, 'src', 'set').and.callThrough();
    await expectAsync(probeMediaDuration(wav(0.5), 'audio')).toBeRejectedWithError('Unexpected object URL scheme');
    expect(setSrc).not.toHaveBeenCalled();
  });
});
