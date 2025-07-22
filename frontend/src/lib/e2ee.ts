export let key: CryptoKey | null = null;
const storageKey = 'cg-msg-key';

export async function initKey() {
  if (typeof localStorage === 'undefined') return;
  const stored = localStorage.getItem(storageKey);
  if (!stored) return;
  try {
    const bytes = Uint8Array.from(atob(stored), (c) => c.charCodeAt(0));
    key = await crypto.subtle.importKey(
      'raw',
      bytes,
      { name: 'AES-GCM', length: 256 },
      false,
      ['encrypt', 'decrypt']
    );
  } catch {
    // ignore parse/import errors
  }
}

export async function setPassword(pw: string) {
  const enc = new TextEncoder();
  const material = await crypto.subtle.importKey(
    'raw',
    enc.encode(pw),
    'PBKDF2',
    false,
    ['deriveKey']
  );
  key = await crypto.subtle.deriveKey(
    { name: 'PBKDF2', salt: enc.encode('cg-msg'), iterations: 100000, hash: 'SHA-256' },
    material,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt']
  );
  if (typeof localStorage !== 'undefined') {
    const raw = await crypto.subtle.exportKey('raw', key);
    const str = btoa(String.fromCharCode(...new Uint8Array(raw)));
    localStorage.setItem(storageKey, str);
  }
}

export function getKey() { return key; }

export async function encryptText(k: CryptoKey, text: string): Promise<string> {
  const enc = new TextEncoder();
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const ct = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, k, enc.encode(text));
  const out = new Uint8Array(iv.length + ct.byteLength);
  out.set(iv, 0); out.set(new Uint8Array(ct), iv.length);
  return btoa(String.fromCharCode(...out));
}

export async function decryptText(k: CryptoKey, data: string): Promise<string> {
  const buf = Uint8Array.from(atob(data), c => c.charCodeAt(0));
  const iv = buf.slice(0, 12);
  const ct = buf.slice(12);
  const plain = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, k, ct);
  return new TextDecoder().decode(plain);
}
