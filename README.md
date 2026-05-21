# S3 Drive

Petite interface web pour gérer un bucket Amazon S3. Backend Go, frontend
rendu côté serveur, pas de SPA. Le but est de tenir face à un audit de
sécurité : surface d'attaque réduite, dépendances minimales, code droit.

TP « Créer une interface pour Amazon S3 » — E-Zy Tech Formation.

## Déploiement live

- **App**     : <https://13-37-241-252.nip.io>
- **Repo**    : <https://github.com/Metsamgit/s3-drive>
- **Hosting** : Amazon Linux 2023 + nginx (TLS Let's Encrypt) → Go en 127.0.0.1:8080,
  unit systemd avec hardening (`NoNewPrivileges`, `ProtectSystem=strict`,
  `ProtectHome`, `PrivateTmp`, `MemoryMax=256M`).

## Fonctionnalités

Côté sujet :

- Lister le contenu d'un bucket avec métadonnées (taille, date)
- Uploader des fichiers (multipart auto pour les gros)
- Télécharger (streamé par le serveur, pas d'URL pré-signée exposée)
- Supprimer
- **Bonus 1** : sélecteur de tous les buckets accessibles au compte
  (`ListBuckets` côté backend, donc plus de blocage CORS comme dans la
  v1 SPA)
- **Bonus 2** : credentials AWS saisis par l'utilisateur, chiffrés en
  mémoire serveur pour la durée de la session, jamais persistés

Plus :

- Navigation par préfixes avec breadcrumbs
- Création de dossier (objet préfixe vide)
- Multi-upload depuis le formulaire

## Stack

Petite par choix. Moins de dépendances = moins de surface CVE.

```
github.com/go-chi/chi/v5                       routeur HTTP
github.com/aws/aws-sdk-go-v2 (config, s3, etc) S3
golang.org/x/time/rate                         token bucket pour le rate-limit
```

Pas de framework UI, pas de bundler, pas de Node. Le `<script>` HTMX
14 KB est servi en local depuis `web/static/` ; aucun CDN tiers.

## Threat model

Ce que je considère comme un attaquant et comment je le freine.

| Vecteur | Mitigation |
|---|---|
| **XSS** | `html/template` (escape contextuel automatique) + CSP strict `default-src 'self'`, pas de inline scripts/styles, `object-src 'none'`, `frame-ancestors 'none'` |
| **CSRF** | Token aléatoire 32 octets par session, contrôle constant-time sur chaque POST. Cookies `SameSite=Strict`. Token pré-login séparé (cookie one-shot) pour défendre le formulaire de connexion. |
| **Clickjacking** | `X-Frame-Options: DENY` + `frame-ancestors 'none'` dans la CSP |
| **MIME sniffing** | `X-Content-Type-Options: nosniff` ; downloads forcés en `Content-Disposition: attachment` |
| **Sniffing du transport** | HSTS un an, `includeSubDomains`, redirection 301 HTTP→HTTPS |
| **Path traversal** | Validateurs stricts sur chaque clé S3 (`internal/validation/validation.go`) : pas de `..`, pas de `//`, charset restreint, longueur ≤ 1024. Filename de multipart filtré via `filepath.Base`. |
| **SSRF** | Aucune URL contrôlée par l'utilisateur n'est fetchée par le serveur |
| **Command injection** | Aucun `exec` ; tout passe par l'AWS SDK |
| **SQLi** | Pas de SQL du tout (session in-memory) |
| **Auth bypass** | Middleware `requireSession` explicite sur toutes les routes protégées, configuré au niveau du routeur |
| **IDOR** | Les clés S3 sont scopées par les credentials de la session ; pas d'ID interne exposé |
| **Brute force AWS keys sur /login** | Rate limit IP dédié sur /login (≈6 req/min) + check côté serveur en pré-flight (HeadBucket/ListBuckets) |
| **Cookie theft** | `HttpOnly` + `Secure` (derrière TLS) + `SameSite=Strict` + `Path=/` |
| **Header injection (Content-Disposition)** | Filename sanitizé : retrait des quotes, backslashes et bytes de contrôle |
| **Memory dump → creds** | Credentials AES-256-GCM en mémoire (clé process-wide, jamais sur disque) ; déchiffrement transitoire au moment de l'appel S3 |
| **Recovery info leak** | Middleware `Recover` qui log la stack côté serveur, renvoie un 500 générique |
| **DoS upload** | `http.MaxBytesReader` borne le body à `MAX_UPLOAD_MB`. nginx `client_max_body_size 110m` en amont. |
| **Slowloris** | `ReadHeaderTimeout: 10s`, `IdleTimeout: 2m`, `MaxHeaderBytes: 64 KB` |
| **Goroutine leak** | Chaque opération AWS a un `context.WithTimeout` ; graceful shutdown borne aussi à 30 s |
| **Long-lived sessions** | Idle TTL 30 min + absolute TTL 8 h ; GC périodique |
| **Stack trace en prod** | `slog` à `info`, jamais d'erreur AWS brute retournée à l'utilisateur — seulement des codes mappés en français |
| **Hard-coded secrets** | Aucun. Tout via env. `SESSION_KEY` doit être passé en prod (sinon warning + ephemeral) |
| **Supply chain** | 5 packages directs, tous sous gouvernance Go/AWS. `go mod tidy` propre. Le binaire est statique. |

Liste lourde mais c'est le but : un auditeur peut cocher les cases une
par une et vérifier dans le code.

## Architecture

```
internal/
├── auth/             session store + CSRF, AES-GCM at rest in memory
├── awsclient/        wrapper minimal sur le SDK S3 v2
├── config/           chargement env vars, validation
├── handlers/         routes (login, files, upload, download, delete, ...)
├── middleware/       security headers, recover, logging, rate limit
└── validation/       validateurs centralisés (bucket, key, region)
web/
├── templates/        html/template (escape auto)
└── static/           css + htmx.min.js
main.go               wiring serveur, timeouts, graceful shutdown
```

Chaque fichier fait une chose. La taille moyenne d'un fichier source
est ~150 lignes ; aucun fichier ne dépasse 250.

## Démarrer en local

```bash
cp .env.example .env
# laisser SESSION_KEY vide pour générer une clé éphémère (warning au boot)
make run
# http://127.0.0.1:8080/login
```

## Production : déploiement EC2 derrière nginx

Le binaire est mono-fichier, build via `make build-linux`. Sur l'EC2 :

1. Installer le binaire dans `/usr/local/bin/s3-drive`
2. `EnvironmentFile=/etc/s3-drive/env` (mode 600) contient le `SESSION_KEY`
3. Unit systemd avec hardening (cf. déploiement live ci-dessus)
4. nginx reverse-proxy `127.0.0.1:8080` + termination TLS Let's Encrypt

Le binaire n'écrit jamais sur disque ; `ProtectSystem=strict` et
`ProtectHome` rendent l'écriture impossible de toute façon.

## Configuration AWS requise

L'utilisateur connecté doit avoir au minimum :

```json
{
  "Version": "2012-10-17",
  "Statement": [
    { "Effect": "Allow", "Action": ["s3:ListAllMyBuckets"], "Resource": "*" },
    { "Effect": "Allow", "Action": ["s3:ListBucket", "s3:GetBucketLocation"], "Resource": "arn:aws:s3:::*" },
    { "Effect": "Allow", "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:AbortMultipartUpload"], "Resource": "arn:aws:s3:::*/*" }
  ]
}
```

Pas de config CORS S3 nécessaire : le backend Go parle à S3, pas le
navigateur.

## Ce qui n'est pas dans le scope (volontairement)

- **OAuth / SSO** : on supporte uniquement le login par credentials AWS
- **Multi-tenant** : une session = un utilisateur AWS
- **Versioning / ACL / tags S3**
- **Pagination listing > 1000 objets** : le SDK retourne `IsTruncated` mais l'UI ne le suit pas encore

## Historique

La v1 de ce projet était une **SPA TypeScript pure** (Vite + AWS SDK v3 navigateur). Elle satisfaisait le bonus 2 (credentials front-end) mais souffrait de la limitation `ListBuckets` (CORS service-level non configurable côté AWS), donc le bonus 1 n'était que partiel.

Cette v2 (Go SSR) est faite pour résister à un audit code/sécurité : tout passe par le backend, ce qui ferme la limitation CORS et permet de raisonner plus simplement sur la surface d'attaque.

L'historique git contient les deux versions.
