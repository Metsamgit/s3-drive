type Kind = "success" | "error" | "info";

export function toast(msg: string, kind: Kind = "info", duration = 3500): void {
  const el = document.createElement("div");
  el.className = `toast ${kind}`;
  el.textContent = msg;
  document.body.appendChild(el);
  setTimeout(() => el.remove(), duration);
}
