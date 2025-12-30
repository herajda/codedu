export function formatDateTime(value: string | number | Date): string {
  const date = ensureDate(value);
  if (!date) {
    return '';
  }

  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  return `${formatDate(date)} ${hours}:${minutes}`;
}

export function formatShortDateTime(value: string | number | Date): string {
  const date = ensureDate(value);
  if (!date) {
    return '';
  }

  const day = String(date.getDate()).padStart(2, '0');
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');

  return `${day}. ${month}. ${hours}:${minutes}`;
}

export function formatDate(value: string | number | Date): string {
  const date = ensureDate(value);
  if (!date) {
    return '';
  }

  const day = String(date.getDate()).padStart(2, '0');
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const year = date.getFullYear();
  return `${day}. ${month}. ${year}`;
}

function ensureDate(value: string | number | Date): Date | null {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}
