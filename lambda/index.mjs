import { S3Client, ListBucketsCommand } from "@aws-sdk/client-s3";

const CORS_HEADERS = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "POST, OPTIONS",
  "Access-Control-Allow-Headers": "content-type",
  "Content-Type": "application/json",
};

export const handler = async (event) => {
  if (event.requestContext?.http?.method === "OPTIONS") {
    return { statusCode: 204, headers: CORS_HEADERS };
  }

  try {
    const body = JSON.parse(event.body || "{}");
    const { accessKeyId, secretAccessKey, region, sessionToken } = body;

    if (!accessKeyId || !secretAccessKey || !region) {
      return {
        statusCode: 400,
        headers: CORS_HEADERS,
        body: JSON.stringify({ error: "Missing credentials" }),
      };
    }

    const client = new S3Client({
      region,
      credentials: { accessKeyId, secretAccessKey, sessionToken },
    });
    const result = await client.send(new ListBucketsCommand({}));
    const buckets = (result.Buckets || []).map((b) => b.Name).filter(Boolean);

    return {
      statusCode: 200,
      headers: CORS_HEADERS,
      body: JSON.stringify({ buckets }),
    };
  } catch (err) {
    return {
      statusCode: 500,
      headers: CORS_HEADERS,
      body: JSON.stringify({ error: err.message || String(err) }),
    };
  }
};
