import {
  S3Client,
  ListBucketsCommand,
  ListObjectsV2Command,
  DeleteObjectCommand,
  GetObjectCommand,
  type _Object,
  type CommonPrefix,
} from "@aws-sdk/client-s3";
import { Upload } from "@aws-sdk/lib-storage";
import { getSignedUrl } from "@aws-sdk/s3-request-presigner";
import type { Credentials } from "./storage.ts";

let client: S3Client | null = null;

export function setupClient(creds: Credentials): S3Client {
  client = new S3Client({
    region: creds.region,
    credentials: {
      accessKeyId: creds.accessKeyId,
      secretAccessKey: creds.secretAccessKey,
      sessionToken: creds.sessionToken,
    },
  });
  return client;
}

export function getClient(): S3Client {
  if (!client) throw new Error("S3 client not initialized");
  return client;
}

export async function listBuckets(): Promise<string[]> {
  const out = await getClient().send(new ListBucketsCommand({}));
  return (out.Buckets ?? []).map((b) => b.Name!).filter(Boolean);
}

export async function bucketAccessible(bucket: string): Promise<boolean> {
  try {
    await getClient().send(
      new ListObjectsV2Command({ Bucket: bucket, MaxKeys: 1 }),
    );
    return true;
  } catch {
    return false;
  }
}

export type ListResult = {
  folders: string[];
  files: Array<{
    key: string;
    name: string;
    size: number;
    lastModified: Date;
  }>;
  truncated: boolean;
  continuationToken?: string;
};

export async function listObjects(
  bucket: string,
  prefix: string,
  continuationToken?: string,
): Promise<ListResult> {
  const out = await getClient().send(
    new ListObjectsV2Command({
      Bucket: bucket,
      Prefix: prefix,
      Delimiter: "/",
      MaxKeys: 1000,
      ContinuationToken: continuationToken,
    }),
  );

  const folders = (out.CommonPrefixes ?? [])
    .map((p: CommonPrefix) => p.Prefix!)
    .filter(Boolean);

  const files = (out.Contents ?? [])
    .filter((o: _Object) => o.Key !== prefix)
    .map((o: _Object) => {
      const key = o.Key!;
      const name = key.slice(prefix.length);
      return {
        key,
        name,
        size: o.Size ?? 0,
        lastModified: o.LastModified ?? new Date(0),
      };
    });

  return {
    folders,
    files,
    truncated: out.IsTruncated ?? false,
    continuationToken: out.NextContinuationToken,
  };
}

export async function uploadFile(
  bucket: string,
  key: string,
  body: File,
  onProgress: (loaded: number, total: number) => void,
): Promise<void> {
  const upload = new Upload({
    client: getClient(),
    params: {
      Bucket: bucket,
      Key: key,
      Body: body,
      ContentType: body.type || "application/octet-stream",
    },
    queueSize: 4,
    partSize: 5 * 1024 * 1024,
    leavePartsOnError: false,
  });

  upload.on("httpUploadProgress", (p) => {
    if (p.loaded != null) onProgress(p.loaded, p.total ?? body.size);
  });

  await upload.done();
}

export async function deleteObject(bucket: string, key: string): Promise<void> {
  await getClient().send(new DeleteObjectCommand({ Bucket: bucket, Key: key }));
}

export async function presignDownload(
  bucket: string,
  key: string,
  expiresIn = 300,
): Promise<string> {
  return getSignedUrl(
    getClient(),
    new GetObjectCommand({ Bucket: bucket, Key: key }),
    { expiresIn },
  );
}
