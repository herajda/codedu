
const BASE: string | undefined = import.meta.env.BAKALARI_BASE_URL;

export const hasBakalari = !!BASE;

export async function login(username: string, password: string) {
  if (!BASE) throw new Error('Bakalari base URL is not configured');
  const form = new URLSearchParams();
  form.set('client_id', 'ANDR');
  form.set('grant_type', 'password');
  form.set('username', username);
  form.set('password', password);
  const res = await fetch(`${BASE}/api/login`, {
    method: 'POST',
    body: form
  });
  if (!res.ok) throw new Error('invalid credentials');
  const { access_token } = await res.json();
  const infoRes = await fetch(`${BASE}/api/3/user`, {
    headers: { Authorization: `Bearer ${access_token}` }
  });
  if (!infoRes.ok) throw new Error('invalid credentials');
  const info = await infoRes.json();
  return { token: access_token as string, info };
}

export async function getAtoms(token: string) {
  if (!BASE) throw new Error('Bakalari base URL is not configured');
  const res = await fetch(`${BASE}/api/3/marking/atoms`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  if (!res.ok) throw new Error('bakalari request failed');
  const data = await res.json();
  return data.Atoms as { Id: string; Name: string }[];
}

export async function getStudents(token: string, atomId: string) {
  if (!BASE) throw new Error('Bakalari base URL is not configured');
  const res = await fetch(`${BASE}/api/3/marking/marks/${atomId}`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  if (!res.ok) throw new Error('bakalari request failed');
  const data = await res.json();
  return data.Students as {
    Id: string;
    ClassId: string;
    FirstName: string;
    MiddleName: string;
    LastName: string;
  }[];
}
