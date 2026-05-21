import {
  setupClient,
  listBuckets,
  bucketAccessible,
  listObjects,
  uploadFile,
  deleteObject,
  presignDownload,
} from "./s3-client.ts";
import {
  type Credentials,
  loadCredentials,
  saveCredentials,
  clearCredentials,
  loadBucket,
  saveBucket,
} from "./storage.ts";
import { openCredentialsModal } from "./ui/credentials-modal.ts";
import { renderFileList, type FileRow } from "./ui/file-list.ts";
import { renderBreadcrumbs } from "./ui/breadcrumbs.ts";
import { openPreview } from "./ui/preview-modal.ts";
import { UploadsPanel } from "./ui/uploads-panel.ts";
import { toast } from "./ui/toast.ts";
import { el, clear } from "./ui/dom.ts";

type AppState = {
  bucket: string | null;
  prefix: string;
};

const state: AppState = {
  bucket: loadBucket(),
  prefix: "",
};

const uploadsPanel = new UploadsPanel();

function buildLayout(): {
  bucketSelect: HTMLSelectElement;
  breadcrumbs: HTMLElement;
  dropzone: HTMLElement;
  statusBar: HTMLElement;
  settingsBtn: HTMLButtonElement;
  refreshBtn: HTMLButtonElement;
  uploadBtn: HTMLButtonElement;
  newFolderBtn: HTMLButtonElement;
  addBucketBtn: HTMLButtonElement;
} {
  const app = document.getElementById("app")!;
  clear(app);

  const bucketSelect = el("select", { id: "bucket-select" }) as HTMLSelectElement;
  const addBucketBtn = el("button", { title: "Ajouter un bucket par nom" }, "+ Bucket") as HTMLButtonElement;
  const refreshBtn = el("button", { class: "icon", title: "Rafraîchir" }, "🔄") as HTMLButtonElement;
  const uploadBtn = el("button", { class: "primary", title: "Uploader des fichiers" }, "+ Upload") as HTMLButtonElement;
  const newFolderBtn = el("button", { title: "Nouveau dossier" }, "+ Dossier") as HTMLButtonElement;
  const settingsBtn = el("button", { class: "icon", title: "Identifiants AWS" }, "⚙") as HTMLButtonElement;

  const header = el("header", {},
    el("h1", {}, "S3 Drive"),
    el("span", { style: "color: var(--muted); font-size: 12px;" }, "Bucket :"),
    bucketSelect,
    addBucketBtn,
    refreshBtn,
    el("span", { class: "spacer" }),
    newFolderBtn,
    uploadBtn,
    settingsBtn,
  );

  const breadcrumbs = el("div", { class: "breadcrumbs" });
  const dropzone = el("div", { class: "dropzone" });
  const statusBar = el("div", { class: "status" }, "Prêt.");

  const main = el("main", {}, breadcrumbs, dropzone);

  app.appendChild(header);
  app.appendChild(main);
  app.appendChild(statusBar);

  return { bucketSelect, breadcrumbs, dropzone, statusBar, settingsBtn, refreshBtn, uploadBtn, newFolderBtn, addBucketBtn };
}

const ui = buildLayout();

async function refresh(): Promise<void> {
  if (!state.bucket) {
    clear(ui.dropzone);
    ui.dropzone.appendChild(
      el("div", { class: "empty" }, "Sélectionnez un bucket pour commencer."),
    );
    renderBreadcrumbs(ui.breadcrumbs, "", () => {});
    setStatus("Aucun bucket sélectionné.");
    return;
  }

  setStatus(`Chargement de ${state.bucket}/${state.prefix}...`);
  renderBreadcrumbs(ui.breadcrumbs, state.prefix, (p) => {
    state.prefix = p;
    void refresh();
  });

  try {
    const result = await listObjects(state.bucket, state.prefix);
    renderFileList(ui.dropzone, state.prefix, result.folders, result.files, {
      onOpenFolder: (folder) => {
        state.prefix = folder;
        void refresh();
      },
      onPreview: handlePreview,
      onDownload: handleDownload,
      onDelete: handleDelete,
    });
    setStatus(
      `${result.folders.length} dossier(s), ${result.files.length} fichier(s)` +
      (result.truncated ? " (tronqué à 1000)" : "")
    );
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    clear(ui.dropzone);
    ui.dropzone.appendChild(el("div", { class: "empty" }, `Erreur : ${msg}`));
    setStatus(`Erreur : ${msg}`);
  }
}

function setStatus(msg: string): void {
  ui.statusBar.textContent = msg;
}

async function loadBucketList(): Promise<void> {
  clear(ui.bucketSelect);
  try {
    const buckets = await listBuckets();
    if (buckets.length === 0) {
      ui.bucketSelect.appendChild(el("option", { value: "" }, "(aucun bucket)"));
      state.bucket = null;
      return;
    }
    for (const name of buckets) {
      ui.bucketSelect.appendChild(el("option", { value: name }, name));
    }
    if (state.bucket && buckets.includes(state.bucket)) {
      ui.bucketSelect.value = state.bucket;
    } else {
      state.bucket = buckets[0];
      ui.bucketSelect.value = state.bucket;
      saveBucket(state.bucket);
    }
  } catch (err) {
    // ListBuckets is a service-level op and does NOT support CORS from a browser.
    // We fall back: keep the previously-saved bucket (if any) and let the user
    // type a bucket name via the manual input button in the header.
    console.warn("ListBuckets blocked (CORS) — falling back to manual entry", err);
    if (state.bucket) {
      ui.bucketSelect.appendChild(el("option", { value: state.bucket }, state.bucket));
      ui.bucketSelect.value = state.bucket;
    } else {
      ui.bucketSelect.appendChild(el("option", { value: "" }, "(saisir un bucket →)"));
    }
    toast(
      "ListBuckets non supporté par S3 en navigateur (CORS service-level). " +
      "Utilise le bouton + Bucket pour saisir un nom.",
      "info",
      6000,
    );
  }
}

async function promptForBucket(): Promise<void> {
  const name = prompt("Nom du bucket S3 :", state.bucket ?? "");
  if (!name) return;
  const trimmed = name.trim();
  if (!trimmed) return;
  setStatus(`Vérification de l'accès à ${trimmed}...`);
  const ok = await bucketAccessible(trimmed);
  if (!ok) {
    toast(`Bucket inaccessible : ${trimmed} (CORS, permissions ou nom invalide).`, "error", 6000);
    setStatus("Bucket inaccessible.");
    return;
  }
  let opt = Array.from(ui.bucketSelect.options).find((o) => o.value === trimmed);
  if (!opt) {
    opt = el("option", { value: trimmed }, trimmed) as HTMLOptionElement;
    ui.bucketSelect.appendChild(opt);
  }
  ui.bucketSelect.value = trimmed;
  state.bucket = trimmed;
  state.prefix = "";
  saveBucket(trimmed);
  await refresh();
}

async function connect(creds: Credentials, remember: boolean): Promise<void> {
  setupClient(creds);
  await loadBucketList();
  saveCredentials(creds, remember);
  state.prefix = "";
  await refresh();
}

ui.bucketSelect.addEventListener("change", () => {
  state.bucket = ui.bucketSelect.value;
  state.prefix = "";
  saveBucket(state.bucket);
  void refresh();
});

ui.refreshBtn.addEventListener("click", () => void refresh());
ui.addBucketBtn.addEventListener("click", () => void promptForBucket());

ui.settingsBtn.addEventListener("click", () => {
  openCredentialsModal(async (creds, remember) => {
    await connect(creds, remember);
    toast("Identifiants mis à jour.", "success");
  });
});

ui.uploadBtn.addEventListener("click", () => {
  const input = document.createElement("input");
  input.type = "file";
  input.multiple = true;
  input.addEventListener("change", () => {
    if (input.files) void uploadFiles(Array.from(input.files));
  });
  input.click();
});

ui.newFolderBtn.addEventListener("click", async () => {
  if (!state.bucket) return;
  const name = prompt("Nom du nouveau dossier :");
  if (!name) return;
  const safe = name.replace(/[/\\]/g, "").trim();
  if (!safe) return;
  const key = state.prefix + safe + "/";
  try {
    const empty = new File([new Blob([])], "", { type: "application/x-directory" });
    await uploadFile(state.bucket, key, empty, () => {});
    toast(`Dossier "${safe}" créé.`, "success");
    void refresh();
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    toast(`Erreur création dossier : ${msg}`, "error");
  }
});

async function uploadFiles(files: File[]): Promise<void> {
  if (!state.bucket) {
    toast("Aucun bucket sélectionné.", "error");
    return;
  }
  const bucket = state.bucket;
  const prefix = state.prefix;

  await Promise.allSettled(
    files.map(async (file) => {
      const id = uploadsPanel.newUpload(file.name, file.size);
      const key = prefix + file.name;
      try {
        await uploadFile(bucket, key, file, (loaded) => {
          uploadsPanel.updateProgress(id, loaded);
        });
        uploadsPanel.markDone(id);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        uploadsPanel.markError(id, msg);
      }
    }),
  );
  await refresh();
}

// Drag & drop
let dragCounter = 0;
ui.dropzone.addEventListener("dragenter", (e) => {
  e.preventDefault();
  dragCounter++;
  ui.dropzone.classList.add("dragging");
});
ui.dropzone.addEventListener("dragover", (e) => {
  e.preventDefault();
  if (e.dataTransfer) e.dataTransfer.dropEffect = "copy";
});
ui.dropzone.addEventListener("dragleave", () => {
  dragCounter--;
  if (dragCounter <= 0) {
    dragCounter = 0;
    ui.dropzone.classList.remove("dragging");
  }
});
ui.dropzone.addEventListener("drop", (e) => {
  e.preventDefault();
  dragCounter = 0;
  ui.dropzone.classList.remove("dragging");
  if (!e.dataTransfer) return;
  const files = Array.from(e.dataTransfer.files);
  if (files.length > 0) void uploadFiles(files);
});

async function handlePreview(file: FileRow): Promise<void> {
  if (!state.bucket) return;
  try {
    const url = await presignDownload(state.bucket, file.key);
    openPreview(file.name, url);
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    toast(`Erreur preview : ${msg}`, "error");
  }
}

async function handleDownload(file: FileRow): Promise<void> {
  if (!state.bucket) return;
  try {
    const url = await presignDownload(state.bucket, file.key);
    const a = document.createElement("a");
    a.href = url;
    a.download = file.name;
    a.rel = "noopener";
    document.body.appendChild(a);
    a.click();
    a.remove();
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    toast(`Erreur téléchargement : ${msg}`, "error");
  }
}

async function handleDelete(file: FileRow): Promise<void> {
  if (!state.bucket) return;
  if (!confirm(`Supprimer "${file.name}" ? Cette action est irréversible.`)) return;
  try {
    await deleteObject(state.bucket, file.key);
    toast(`"${file.name}" supprimé.`, "success");
    void refresh();
  } catch (err) {
    const errObj = err as { name?: string; Code?: string; message?: string };
    if (errObj.name === "AccessDenied" || errObj.Code === "AccessDenied") {
      toast("Accès refusé : votre IAM n'a pas s3:DeleteObject.", "error", 6000);
    } else {
      toast(`Erreur suppression : ${errObj.message ?? String(err)}`, "error");
    }
  }
}

// Boot
async function boot(): Promise<void> {
  const existing = loadCredentials();
  if (existing) {
    try {
      await connect(existing, true);
      return;
    } catch {
      clearCredentials();
    }
  }
  openCredentialsModal(connect);
}

void boot();
