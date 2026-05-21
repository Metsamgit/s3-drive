import { el, clear } from "./dom.ts";
import { canPreview } from "./preview-modal.ts";

export type FileRow = {
  key: string;
  name: string;
  size: number;
  lastModified: Date;
};

export type FileListHandlers = {
  onOpenFolder: (prefix: string) => void;
  onPreview: (file: FileRow) => void;
  onDownload: (file: FileRow) => void;
  onDelete: (file: FileRow) => void;
};

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 ** 2) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 ** 3) return `${(n / 1024 ** 2).toFixed(1)} MB`;
  return `${(n / 1024 ** 3).toFixed(1)} GB`;
}

function formatDate(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getDate())}/${pad(d.getMonth() + 1)}/${d.getFullYear()} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function iconFor(name: string): string {
  const ext = name.split(".").pop()?.toLowerCase() ?? "";
  if (["jpg", "jpeg", "png", "gif", "webp", "svg", "bmp"].includes(ext)) return "IMG";
  if (ext === "pdf") return "PDF";
  if (["doc", "docx", "txt", "md", "rtf"].includes(ext)) return "DOC";
  if (["xls", "xlsx", "csv"].includes(ext)) return "XLS";
  if (["zip", "tar", "gz", "rar", "7z"].includes(ext)) return "ZIP";
  if (["mp4", "mov", "avi", "mkv", "webm"].includes(ext)) return "VID";
  if (["mp3", "wav", "ogg", "flac"].includes(ext)) return "MP3";
  if (["js", "ts", "py", "go", "rs", "java", "c", "cpp", "h", "json", "html", "css"].includes(ext)) return "COD";
  return "FIL";
}

function folderName(prefix: string, parent: string): string {
  return prefix.slice(parent.length).replace(/\/$/, "");
}

export function renderFileList(
  container: HTMLElement,
  parentPrefix: string,
  folders: string[],
  files: FileRow[],
  handlers: FileListHandlers,
): void {
  clear(container);

  if (folders.length === 0 && files.length === 0) {
    container.appendChild(
      el("div", { class: "empty" },
        "Dossier vide. Glissez-déposez des fichiers ici pour les uploader."),
    );
    return;
  }

  const tbody = el("tbody");

  for (const folder of folders) {
    const name = folderName(folder, parentPrefix);
    const row = el("tr",
      { onclick: () => handlers.onOpenFolder(folder) },
      el("td", { class: "name" },
        el("span", { class: "icon" }, "DIR"),
        name + "/",
      ),
      el("td", {}, "—"),
      el("td", {}, "—"),
      el("td", { class: "actions" }, ""),
    );
    tbody.appendChild(row);
  }

  for (const file of files) {
    const previewBtn = canPreview(file.name)
      ? el("button", {
          class: "icon", title: "Aperçu",
          onclick: (e: Event) => {
            e.stopPropagation();
            handlers.onPreview(file);
          },
        }, "Voir")
      : null;

    const downloadBtn = el("button", {
      class: "icon", title: "Télécharger",
      onclick: (e: Event) => {
        e.stopPropagation();
        handlers.onDownload(file);
      },
    }, "DL");

    const deleteBtn = el("button", {
      class: "icon", title: "Supprimer",
      onclick: (e: Event) => {
        e.stopPropagation();
        handlers.onDelete(file);
      },
    }, "Suppr");

    const nameCell = el("td", {
      class: "name",
      onclick: () => {
        if (canPreview(file.name)) handlers.onPreview(file);
        else handlers.onDownload(file);
      },
    },
      el("span", { class: "icon" }, iconFor(file.name)),
      file.name,
    );

    const actionsCell = el("td", { class: "actions" });
    if (previewBtn) actionsCell.appendChild(previewBtn);
    actionsCell.appendChild(downloadBtn);
    actionsCell.appendChild(deleteBtn);

    const row = el("tr",
      {},
      nameCell,
      el("td", {}, formatBytes(file.size)),
      el("td", {}, formatDate(file.lastModified)),
      actionsCell,
    );
    tbody.appendChild(row);
  }

  const table = el("table", { class: "files" },
    el("thead",
      {},
      el("tr", {},
        el("th", {}, "Nom"),
        el("th", {}, "Taille"),
        el("th", {}, "Modifié"),
        el("th", {}, ""),
      ),
    ),
    tbody,
  );

  container.appendChild(table);
}
