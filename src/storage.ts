export type Credentials = {
  accessKeyId: string;
  secretAccessKey: string;
  region: string;
  sessionToken?: string;
};

const CREDS_KEY = "s3-drive:credentials";
const BUCKET_KEY = "s3-drive:bucket";
const REMEMBER_KEY = "s3-drive:remember";

export function loadCredentials(): Credentials | null {
  const remembered = sessionStorage.getItem(CREDS_KEY) ?? localStorage.getItem(CREDS_KEY);
  if (!remembered) return null;
  try {
    return JSON.parse(remembered) as Credentials;
  } catch {
    return null;
  }
}

export function saveCredentials(creds: Credentials, remember: boolean): void {
  const json = JSON.stringify(creds);
  if (remember) {
    localStorage.setItem(CREDS_KEY, json);
    localStorage.setItem(REMEMBER_KEY, "1");
    sessionStorage.removeItem(CREDS_KEY);
  } else {
    sessionStorage.setItem(CREDS_KEY, json);
    localStorage.removeItem(CREDS_KEY);
    localStorage.removeItem(REMEMBER_KEY);
  }
}

export function isRemembering(): boolean {
  return localStorage.getItem(REMEMBER_KEY) === "1";
}

export function clearCredentials(): void {
  sessionStorage.removeItem(CREDS_KEY);
  localStorage.removeItem(CREDS_KEY);
  localStorage.removeItem(REMEMBER_KEY);
}

export function loadBucket(): string | null {
  return localStorage.getItem(BUCKET_KEY);
}

export function saveBucket(name: string): void {
  localStorage.setItem(BUCKET_KEY, name);
}
