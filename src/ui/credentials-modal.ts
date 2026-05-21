import { type Credentials, isRemembering, loadCredentials, loadBucket } from "../storage.ts";
import { el } from "./dom.ts";

const AWS_REGIONS = [
  "eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1", "eu-north-1",
  "us-east-1", "us-east-2", "us-west-1", "us-west-2",
  "ap-northeast-1", "ap-southeast-1", "ap-southeast-2", "ap-south-1",
];

export function openCredentialsModal(
  onSubmit: (creds: Credentials, bucket: string | null, remember: boolean) => Promise<void>,
): void {
  const existing = loadCredentials();
  const existingBucket = loadBucket();
  const remembered = isRemembering();

  const akInput = el("input", { id: "ak", type: "text", autocomplete: "off", placeholder: "AKIA..." });
  const skInput = el("input", { id: "sk", type: "password", autocomplete: "off" });
  const regionSelect = el("select", { id: "region" },
    ...AWS_REGIONS.map((r) => el("option", { value: r }, r)),
  );
  const bucketInput = el("input", {
    id: "bucket", type: "text", autocomplete: "off",
    placeholder: "nom-du-bucket (sera vérifié à la connexion)",
  });
  const tokenInput = el("input", {
    id: "token", type: "password", autocomplete: "off",
    placeholder: "Pour les credentials temporaires STS",
  });
  const rememberInput = el("input", { id: "remember", type: "checkbox" });
  const errorEl = el("div", { class: "error", style: "display: none;" });
  const cancelBtn = el("button", { id: "cancel" }, "Annuler");
  const submitBtn = el("button", { id: "submit", class: "primary" }, "Se connecter");

  if (existing) {
    akInput.value = existing.accessKeyId;
    skInput.value = existing.secretAccessKey;
    regionSelect.value = existing.region;
    tokenInput.value = existing.sessionToken ?? "";
  }
  if (existingBucket) bucketInput.value = existingBucket;
  rememberInput.checked = remembered;
  if (!existing) cancelBtn.style.display = "none";

  const modal = el("div", { class: "modal", role: "dialog" },
    el("h2", {}, "Connexion AWS"),
    el("p", { style: "color: var(--muted); font-size: 12px; margin: 0 0 16px 0;" },
      "Les identifiants ne quittent jamais votre navigateur. Aucun serveur tiers n'est impliqué."),
    el("div", { class: "field" },
      el("label", { for: "ak" }, "Access Key ID"),
      akInput,
    ),
    el("div", { class: "field" },
      el("label", { for: "sk" }, "Secret Access Key"),
      skInput,
    ),
    el("div", { class: "field" },
      el("label", { for: "region" }, "Région"),
      regionSelect,
    ),
    el("div", { class: "field" },
      el("label", { for: "bucket" }, "Bucket S3"),
      bucketInput,
    ),
    el("div", { class: "field" },
      el("label", { for: "token" }, "Session Token (optionnel)"),
      tokenInput,
    ),
    el("div", { class: "checkbox-row" },
      rememberInput,
      el("label", { for: "remember", style: "margin: 0; color: var(--text);" },
        "Mémoriser dans ce navigateur (localStorage)"),
    ),
    errorEl,
    el("div", { class: "actions" }, cancelBtn, submitBtn),
  );

  const backdrop = el("div", { class: "modal-backdrop" }, modal);
  document.body.appendChild(backdrop);

  const close = () => backdrop.remove();
  cancelBtn.addEventListener("click", close);

  const showError = (msg: string) => {
    errorEl.textContent = msg;
    errorEl.style.display = "block";
  };

  submitBtn.addEventListener("click", async () => {
    errorEl.style.display = "none";
    const ak = akInput.value.trim();
    const sk = skInput.value.trim();
    const region = regionSelect.value;
    const token = tokenInput.value.trim();
    const bucket = bucketInput.value.trim();

    if (!ak || !sk || !region) {
      showError("Access Key, Secret et région sont requis.");
      return;
    }

    submitBtn.disabled = true;
    submitBtn.textContent = "Connexion...";

    try {
      const creds: Credentials = {
        accessKeyId: ak,
        secretAccessKey: sk,
        region,
        sessionToken: token || undefined,
      };
      await onSubmit(creds, bucket || null, rememberInput.checked);
      close();
    } catch (err) {
      submitBtn.disabled = false;
      submitBtn.textContent = "Se connecter";
      const msg = err instanceof Error ? err.message : String(err);
      showError(`Échec de connexion : ${msg}`);
    }
  });

  akInput.focus();
}
