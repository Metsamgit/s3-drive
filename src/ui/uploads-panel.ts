import { el, clear } from "./dom.ts";

export type UploadState = {
  id: string;
  name: string;
  size: number;
  loaded: number;
  status: "uploading" | "done" | "error";
  error?: string;
};

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 ** 2) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 ** 3) return `${(n / 1024 ** 2).toFixed(1)} MB`;
  return `${(n / 1024 ** 3).toFixed(1)} GB`;
}

export class UploadsPanel {
  private panel: HTMLElement | null = null;
  private items: Map<string, UploadState> = new Map();
  private listEl: HTMLElement | null = null;
  private titleEl: HTMLElement | null = null;
  private nextId = 0;

  private ensurePanel(): HTMLElement {
    if (this.panel) return this.panel;
    this.listEl = el("div", { class: "upload-list" });
    this.titleEl = el("span", {}, "Transferts");
    const closeBtn = el("button", { class: "icon", title: "Fermer" }, "×");
    closeBtn.addEventListener("click", () => this.clear());
    this.panel = el("div", { class: "uploads" },
      el("h3", {}, this.titleEl, closeBtn),
      this.listEl,
    );
    document.body.appendChild(this.panel);
    return this.panel;
  }

  newUpload(name: string, size: number): string {
    this.ensurePanel();
    const id = `u${this.nextId++}`;
    this.items.set(id, { id, name, size, loaded: 0, status: "uploading" });
    this.render();
    return id;
  }

  updateProgress(id: string, loaded: number): void {
    const item = this.items.get(id);
    if (!item) return;
    item.loaded = loaded;
    this.render();
  }

  markDone(id: string): void {
    const item = this.items.get(id);
    if (!item) return;
    item.status = "done";
    item.loaded = item.size;
    this.render();
    setTimeout(() => {
      this.items.delete(id);
      this.render();
    }, 4000);
  }

  markError(id: string, msg: string): void {
    const item = this.items.get(id);
    if (!item) return;
    item.status = "error";
    item.error = msg;
    this.render();
  }

  private clear(): void {
    this.items.clear();
    if (this.panel) {
      this.panel.remove();
      this.panel = null;
    }
  }

  private render(): void {
    if (!this.listEl || !this.titleEl) return;
    clear(this.listEl);

    if (this.items.size === 0) {
      if (this.panel) {
        this.panel.remove();
        this.panel = null;
      }
      return;
    }

    const total = this.items.size;
    const done = [...this.items.values()].filter((i) => i.status === "done").length;
    this.titleEl.textContent = `Transferts (${done}/${total})`;

    for (const item of this.items.values()) {
      const pct = item.size > 0 ? Math.min(100, (item.loaded / item.size) * 100) : 0;
      const statusText = item.status === "error"
        ? item.error ?? "Erreur"
        : `${formatBytes(item.loaded)} / ${formatBytes(item.size)}`;

      const itemEl = el("div", { class: `upload-item ${item.status}` },
        el("span", { class: "name", title: item.name }, item.name),
        el("div", { class: "bar" },
          el("div", { class: "fill", style: `width: ${pct}%` }),
        ),
        el("div", { class: "meta" },
          el("span", {}, statusText),
          el("span", {}, `${pct.toFixed(0)}%`),
        ),
      );
      this.listEl.appendChild(itemEl);
    }
  }
}
