# S3 Drive

Interface web type "drive" pour Amazon S3 — SPA TypeScript sans backend.
Toutes les requêtes S3 sont signées directement dans le navigateur ; vos
identifiants AWS ne quittent jamais la machine cliente.

## Fonctionnalités

- Lister le contenu d'un bucket avec métadonnées (taille, date, type)
- Upload par drag & drop multi-fichiers, avec barre de progression
- Upload multipart automatique pour les gros fichiers (parts de 5 MB en parallèle)
- Téléchargement via URL pré-signées (expirent en 5 min)
- Suppression avec confirmation
- **Bonus 1** : sélection parmi tous les buckets accessibles au compte
- **Bonus 2** : credentials AWS saisis côté front-end (jamais transmis à un serveur tiers)
- Navigation hiérarchique par préfixes (dossiers) avec breadcrumbs
- Prévisualisation in-app pour images et PDF
- Création de "dossier" (objet préfixe vide)

## Stack

- **Vite** + **TypeScript** (SPA, pas de framework UI)
- **@aws-sdk/client-s3** v3 (modulaire, tree-shakable)
- **@aws-sdk/lib-storage** pour l'upload multipart automatique
- **@aws-sdk/s3-request-presigner** pour les URL de téléchargement

Aucun backend. Les fichiers statiques générés par `npm run build` peuvent
être servis par n'importe quel hébergeur statique (S3 + CloudFront, Netlify,
Vercel, Cloudflare Pages, GitHub Pages, etc.).

## Démarrer

```bash
npm install
npm run dev          # serveur de dev sur http://localhost:5173
npm run build        # build de production dans dist/
npm run preview      # preview du build
```

Au premier chargement, une fenêtre demande les identifiants AWS. Cochez
"Mémoriser" pour les conserver dans le localStorage du navigateur, sinon
ils ne sont gardés que pour la session courante.

## Configuration AWS

### 1. Politique IAM minimale

Attachez cette politique à l'utilisateur IAM dont vous utiliserez les
identifiants. Les actions sont restreintes au strict nécessaire.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:ListAllMyBuckets"],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": ["s3:ListBucket", "s3:GetBucketLocation"],
      "Resource": "arn:aws:s3:::*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:AbortMultipartUpload"
      ],
      "Resource": "arn:aws:s3:::*/*"
    }
  ]
}
```

Pour restreindre à un seul bucket, remplacez `arn:aws:s3:::*` par
`arn:aws:s3:::nom-du-bucket` et `arn:aws:s3:::*/*` par
`arn:aws:s3:::nom-du-bucket/*`.

### 2. Configuration CORS du bucket

Sans CORS, le navigateur bloquera toutes les requêtes vers S3. Sur chaque
bucket à utiliser, ajoutez cette configuration (console AWS → bucket →
Permissions → Cross-origin resource sharing) :

```json
[
  {
    "AllowedHeaders": ["*"],
    "AllowedMethods": ["GET", "PUT", "POST", "DELETE", "HEAD"],
    "AllowedOrigins": [
      "http://localhost:5173",
      "https://votre-domaine-de-prod.example"
    ],
    "ExposeHeaders": ["ETag"],
    "MaxAgeSeconds": 3000
  }
]
```

Remplacez `https://votre-domaine-de-prod.example` par l'URL où vous
hébergerez l'application en production. Pour un test rapide vous pouvez
mettre `"*"`, mais ne le laissez pas en production.

### 3. (Optionnel) Bloquer l'accès public

Cette application n'a pas besoin que les objets soient publics : tous les
téléchargements passent par des URL pré-signées. Gardez "Block all public
access" activé sur le bucket.

## Architecture

```
src/
├── main.ts                  Orchestration de l'app
├── s3-client.ts             Wrapper AWS SDK (list, upload, delete, presign)
├── storage.ts               Persistance creds + bucket (localStorage / sessionStorage)
├── style.css                Tous les styles
└── ui/
    ├── dom.ts               Helper de création DOM (alternative à innerHTML)
    ├── credentials-modal.ts Saisie des identifiants AWS
    ├── bucket-selector.ts   (inline dans main.ts)
    ├── breadcrumbs.ts       Navigation par préfixe
    ├── file-list.ts         Tableau des fichiers/dossiers
    ├── preview-modal.ts     Aperçu images/PDF
    ├── uploads-panel.ts     Panneau de progression des uploads
    └── toast.ts             Notifications
```

## Sécurité

- Les credentials sont stockés dans `localStorage` ou `sessionStorage` du
  navigateur. Sur une machine partagée, **ne cochez pas "Mémoriser"**.
- L'application est statique : un attaquant qui obtient les identifiants
  via XSS aurait les mêmes droits que l'utilisateur. Évitez d'injecter du
  HTML utilisateur dans l'app — toute construction DOM passe par le helper
  `el()` qui utilise `textContent` plutôt que `innerHTML`.
- En production, servez l'app derrière HTTPS et configurez le CORS du
  bucket pour n'autoriser que votre domaine.
- Les URL pré-signées expirent par défaut en 5 minutes.

## Limites connues

- L'app reste sur la même région que l'utilisateur ; les buckets dans une
  région différente fonctionnent grâce au mode "auto" du SDK v3 mais la
  première requête peut entraîner une redirection.
- Le listing est limité à 1000 entrées par page. La pagination n'est pas
  encore implémentée dans l'UI (le SDK retourne `IsTruncated`).
- Pas de gestion du versioning S3, ACL, tags, ou métadonnées custom.

## Livrable

Le code source est publié sur le dépôt Git indiqué par le formateur.
