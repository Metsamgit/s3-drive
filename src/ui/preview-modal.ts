import { el } from "./dom.ts";

const IMAGE_EXT = new Set(["jpg", "jpeg", "png", "gif", "webp", "svg", "bmp"]);
const PDF_EXT = new Set(["pdf"]);

export function canPreview(name: string): boolean {
  const ext = name.split(".").pop()?.toLowerCase() ?? "";
  return IMAGE_EXT.has(ext) || PDF_EXT.has(ext);
}

export function openPreview(name: string, url: string): void {
  const ext = name.split(".").pop()?.toLowerCase() ?? "";
  const isImage = IMAGE_EXT.has(ext);

  const closeBtn = el("button", {}, "Fermer");
  const downloadBtn = el("a", {
    href: url, download: name, target: "_blank", rel: "noopener",
    class: "button-link",
  });
  // anchor styled as button — we apply class for visual via wrapping span/button
  const downloadWrap = el("button", { class: "primary" }, "Télécharger");
  downloadWrap.addEventListener("click", () => {
    downloadBtn.click();
  });
  downloadBtn.style.display = "none";

  const content = isImage
    ? el("img", { src: url, alt: name })
    : el("iframe", { src: url, title: name });

  const modal = el("div", { class: "modal" },
    el("h2", {}, name),
    el("div", { class: "preview-content" }, content),
    el("div", { class: "actions" }, downloadBtn, downloadWrap, closeBtn),
  );

  const backdrop = el("div", { class: "modal-backdrop" }, modal);
  document.body.appendChild(backdrop);

  const close = () => backdrop.remove();
  closeBtn.addEventListener("click", close);
  backdrop.addEventListener("click", (e) => {
    if (e.target === backdrop) close();
  });
  document.addEventListener("keydown", function onKey(e) {
    if (e.key === "Escape") {
      close();
      document.removeEventListener("keydown", onKey);
    }
  });
}
