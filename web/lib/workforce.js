export const stations = [
  ["kitchen", "Kitchen"],
  ["prep", "Preparation"],
  ["service", "Service"],
  ["cashier", "Kasir"],
  ["beverage", "Beverage"],
  ["steward", "Steward"],
];

export function isoDate(date = new Date()) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 10);
}

export function addDaysISO(days, date = new Date()) {
  const next = new Date(date);
  next.setDate(next.getDate() + days);
  return isoDate(next);
}

export function formatDate(value) {
  if (!value) return "-";
  const date = new Date(`${String(value).slice(0, 10)}T00:00:00`);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("id-ID", { weekday: "short", day: "numeric", month: "short" }).format(date);
}

export function formatTime(value) {
  if (!value) return "-";
  if (/^\d{2}:\d{2}/.test(value)) return value.slice(0, 5);
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat("id-ID", { hour: "2-digit", minute: "2-digit", hour12: false }).format(date);
}

export function activeAttendance(rows = []) {
  return rows.find((row) => !row.clock_out) || null;
}
