export function redirectTo(path) {
  if (typeof window !== "undefined") {
    window.location.replace(path);
  }
}
