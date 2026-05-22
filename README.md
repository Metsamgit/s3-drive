# S3 Drive

Petite app web pour gérer des fichiers dans un bucket Amazon S3.
TP "Créer une interface pour Amazon S3" — E-Zy Tech Formation.

## Live

- App : https://13-37-241-252.nip.io
- Hébergée sur une EC2 (Amazon Linux 2023 + nginx + Let's Encrypt)

## Ce que ça fait

- Lister les fichiers d'un bucket (taille, date)
- Upload (multipart auto pour les gros fichiers)
- Download
- Supprimer
- Naviguer dans les dossiers, en créer
- Bonus 1 : sélecteur des buckets du compte connecté
- Bonus 2 : les credentials AWS sont saisis à la connexion et gardés en mémoire serveur le temps de la session

## Stack

Backend Go qui rend le HTML côté serveur, et HTMX pour quelques bouts
interactifs (delete sans reload, etc). Pas de framework front, pas de
bundler.

```
github.com/go-chi/chi/v5
github.com/aws/aws-sdk-go-v2
golang.org/x/time/rate
```

## Lancer en local

```
cp .env.example .env
make run
```

Puis http://127.0.0.1:8080/login.

## Policy IAM minimale

```json
{
  "Version": "2012-10-17",
  "Statement": [
    { "Effect": "Allow", "Action": ["s3:ListAllMyBuckets"], "Resource": "*" },
    { "Effect": "Allow", "Action": ["s3:ListBucket"], "Resource": "arn:aws:s3:::*" },
    { "Effect": "Allow", "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"], "Resource": "arn:aws:s3:::*/*" }
  ]
}
```

## Notes

Première version en SPA TypeScript (dispo dans l'historique git). Passé en
backend Go ensuite, à la fois pour faire marcher le bonus 1 (le navigateur
ne peut pas appeler `ListBuckets` à cause de CORS) et pour avoir un code
plus simple à relire pour l'exercice de code review.
