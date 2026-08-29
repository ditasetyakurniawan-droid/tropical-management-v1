const roleLabels = {
  admin: "Owner",
  auditor: "PIC",
  staff: "Karyawan",
};

const statusLabels = {
  open: "Terbuka",
  in_progress: "Diproses",
  resolved: "Diselesaikan",
  verified: "Terverifikasi",
  closed: "Ditutup",
  scheduled: "Terjadwal",
  present: "Sedang Bertugas",
  completed: "Selesai",
  pending: "Menunggu",
  approved: "Disetujui",
  rejected: "Ditolak",
  done: "Selesai",
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

const stationLabels = {
  kitchen: "Kitchen",
  prep: "Preparation",
  service: "Service",
  cashier: "Kasir",
  beverage: "Beverage",
  steward: "Steward",
};

const timeOffTypeLabels = {
  leave: "Cuti",
  sick: "Sakit",
  permission: "Izin",
};

const priorityLabels = {
  low: "Rendah",
  normal: "Normal",
  high: "Tinggi",
};

export function stationLabel(value) {
  return stationLabels[value] || value || "-";
}

export function timeOffTypeLabel(value) {
  return timeOffTypeLabels[value] || value || "-";
}

export function priorityLabel(value) {
  return priorityLabels[value] || value || "-";
}
