/**
 * Puts text on the clipboard. Resolves true when it worked, false when the browser
 * refused (the Clipboard API needs a secure page and a user gesture; the fallback
 * uses a hidden text box for older browsers). It never throws.
 */
export async function copyText(text: string, doc: Document = document): Promise<boolean> {
  try {
    await doc.defaultView!.navigator.clipboard.writeText(text);
    return true;
  } catch {
    try {
      const area = doc.createElement('textarea');
      area.value = text;
      area.setAttribute('readonly', '');
      area.style.position = 'fixed';
      area.style.opacity = '0';
      doc.body.appendChild(area);
      area.select();
      const ok = doc.execCommand('copy');
      doc.body.removeChild(area);
      return ok;
    } catch {
      return false;
    }
  }
}
