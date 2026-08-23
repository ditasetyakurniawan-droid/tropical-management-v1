const roleLabels = {
  admin: "Admin",
  auditor: "Auditor",
  staff: "Staf",
};

const statusLabels = {
  open: "Terbuka",
  in_progress: "Diproses",
  resolved: "Diselesaikan",
  verified: "Terverifikasi",
  closed: "Ditutup",
};

const severityLabels = {
  low: "Rendah",
  medium: "Sedang",
  high: "Tinggi",
  critical: "Kritis",
};

const channelLabels = {
  "dine-in": "Makan di Tempat",
  takeaway: "Bawa Pulang",
  gofood: "GoFood",
  grabfood: "GrabFood",
  shopeefood: "ShopeeFood",
  website: "Situs Web",
  corporate: "Korporat",
};

export function roleLabel(value) {
  return roleLabels[value] || value || "-";
}

export function statusLabel(value) {
  return statusLabels[value] || String(value || "-").replaceAll("_", " ");
}

export function severityLabel(value) {
  return severityLabels[value] || value || "-";
}

export function channelLabel(value) {
  return channelLabels[value] || value || "-";
}
