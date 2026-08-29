export const ROLE_ADMIN = "admin";
export const ROLE_AUDITOR = "auditor";
export const ROLE_STAFF = "staff";

export function personaForRole(role) {
  if (role === ROLE_ADMIN) return "owner";
  if (role === ROLE_AUDITOR) return "pic";
  return "employee";
}

export function isManagerRole(role) {
  return role === ROLE_ADMIN || role === ROLE_AUDITOR;
}

export function navigationForRole(role) {
  const common = [
    { href: "/", label: "Beranda" },
    { href: "/workforce", label: role === ROLE_STAFF ? "Hari & Shift" : "Tim & Shift" },
  ];

  if (role === ROLE_STAFF) {
    return [
      ...common,
      { href: "/sales", label: "Penjualan" },
      { href: "/chat", label: "Chat Tim" },
    ];
  }

  const operations = [
    { href: "/audits", label: "Kualitas" },
    { href: "/inventory", label: "Stok" },
    { href: "/sales", label: "Penjualan" },
    { href: "/chat", label: "Chat Tim" },
  ];

  if (role === ROLE_ADMIN) {
    operations.push({ href: "/users", label: "Akses Tim" });
  }

  return [...common, ...operations];
}
