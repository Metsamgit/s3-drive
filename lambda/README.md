# Lambda `list-buckets` (tentative)

Cette Lambda résout la limitation CORS de `ListBuckets` (qui empêche le bonus 1
d'être pleinement satisfait depuis une SPA pure).

## Pourquoi cette Lambda ?

`s3:ListBuckets` est une opération **service-level** : son URL est
`https://s3.amazonaws.com/` (pas de bucket dans l'URL). Or la config CORS sur S3
s'applique **par bucket**, jamais au niveau service. Donc impossible de l'appeler
depuis un navigateur sans backend.

## Comment ça marche

Le frontend POSTe les credentials AWS au endpoint Lambda → la Lambda appelle
`ListBuckets` avec ces credentials → renvoie la liste avec headers CORS OK.

Les credentials transitent par la Lambda mais ne sont jamais stockés (la Lambda
les utilise pour 1 seul appel S3 et les jette).

## Déploiement

```bash
cd lambda
zip function.zip index.mjs

# 1. Créer un rôle IAM pour la Lambda (trust policy lambda.amazonaws.com)
aws iam create-role \
  --role-name s3-drive-lambda-role \
  --assume-role-policy-document file://trust-policy.json

aws iam attach-role-policy \
  --role-name s3-drive-lambda-role \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole

# 2. Créer la fonction
aws lambda create-function \
  --function-name s3-drive-list-buckets \
  --runtime nodejs20.x \
  --role arn:aws:iam::ACCOUNT:role/s3-drive-lambda-role \
  --handler index.handler \
  --zip-file fileb://function.zip \
  --timeout 10 --memory-size 256

# 3. Function URL publique avec CORS
aws lambda create-function-url-config \
  --function-name s3-drive-list-buckets \
  --auth-type NONE \
  --cors '{"AllowOrigins":["*"],"AllowMethods":["POST"],"AllowHeaders":["content-type"]}'

aws lambda add-permission \
  --function-name s3-drive-list-buckets \
  --statement-id FunctionURLAllowPublicAccess \
  --action lambda:InvokeFunctionUrl \
  --principal "*" \
  --function-url-auth-type NONE
```

## Problème rencontré pendant le TP

Sur le compte AWS utilisé pour le TP, le Function URL retourne **HTTP 403
Forbidden** malgré une configuration correcte (vérifiée via `get-function-url-config`
et `get-policy`). Cela suggère une **Service Control Policy** (SCP) ou un
**permission boundary** au niveau du compte qui bloque les Function URLs en auth
`NONE`.

Sans accès à la console IAM pour debugger les SCPs (ou pour activer Function
URLs publiques dans les guardrails), cette voie a été abandonnée.

**Workaround actuel dans l'app** : saisie manuelle du nom de bucket dans la modal
de connexion. Couvre 90% du besoin du bonus 1.

## Pistes pour faire marcher la Lambda

- Vérifier les SCPs sur le compte/organisation
- Tester avec `--auth-type AWS_IAM` (le client devra signer les requêtes, plus
  complexe côté frontend mais évite le 403)
- Utiliser API Gateway HTTP API à la place (parfois moins de restrictions)
