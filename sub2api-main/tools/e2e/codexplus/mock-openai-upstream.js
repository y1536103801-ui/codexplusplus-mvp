const http = require("node:http");
const crypto = require("node:crypto");

const args = new Map();
for (let i = 2; i < process.argv.length; i += 2) {
  args.set(process.argv[i], process.argv[i + 1]);
}

const host = args.get("--host") || process.env.CODEXPLUS_MOCK_OPENAI_HOST || "0.0.0.0";
const port = Number(args.get("--port") || process.env.CODEXPLUS_MOCK_OPENAI_PORT || 18081);

function sendJson(res, status, body, requestId) {
  res.writeHead(status, {
    "content-type": "application/json",
    "x-request-id": requestId,
    "openai-processing-ms": "7",
  });
  res.end(JSON.stringify(body));
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    req.on("data", (chunk) => chunks.push(chunk));
    req.on("end", () => resolve(Buffer.concat(chunks).toString("utf8")));
    req.on("error", reject);
  });
}

const server = http.createServer(async (req, res) => {
  const requestId = `mock_${crypto.randomUUID()}`;

  if (req.method === "GET" && req.url === "/health") {
    sendJson(res, 200, { status: "ok", service: "codexplus-mock-openai" }, requestId);
    return;
  }

  if (req.method !== "POST") {
    sendJson(res, 404, { error: { type: "not_found_error", message: "not found" } }, requestId);
    return;
  }

  const rawBody = await readBody(req);
  let parsed = {};
  try {
    parsed = rawBody ? JSON.parse(rawBody) : {};
  } catch {
    sendJson(res, 400, { error: { type: "invalid_request_error", message: "invalid json" } }, requestId);
    return;
  }

  const model = typeof parsed.model === "string" && parsed.model.trim() ? parsed.model : "gpt-5-mini";

  if (req.url === "/v1/responses") {
    sendJson(res, 200, {
      id: `resp_${crypto.randomUUID()}`,
      object: "response",
      created_at: Math.floor(Date.now() / 1000),
      status: "completed",
      model,
      output: [
        {
          id: `msg_${crypto.randomUUID()}`,
          type: "message",
          status: "completed",
          role: "assistant",
          content: [{ type: "output_text", text: "pong" }],
        },
      ],
      usage: {
        input_tokens: 8,
        output_tokens: 1,
        total_tokens: 9,
      },
    }, requestId);
    return;
  }

  if (req.url === "/v1/chat/completions") {
    sendJson(res, 200, {
      id: `chatcmpl_${crypto.randomUUID()}`,
      object: "chat.completion",
      created: Math.floor(Date.now() / 1000),
      model,
      choices: [{ index: 0, message: { role: "assistant", content: "pong" }, finish_reason: "stop" }],
      usage: { prompt_tokens: 8, completion_tokens: 1, total_tokens: 9 },
    }, requestId);
    return;
  }

  sendJson(res, 404, { error: { type: "not_found_error", message: "not found" } }, requestId);
});

server.listen(port, host, () => {
  console.log(`codexplus mock OpenAI upstream listening on http://${host}:${port}`);
});
