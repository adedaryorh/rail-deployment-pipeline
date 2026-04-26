const http = require("http");

const PORT = process.env.PORT || 3000;
const VERSION = process.env.APP_VERSION || "1.0.0";

const server = http.createServer((req, res) => {
  if (req.url === "/health") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ status: "ok", version: VERSION }));
    return;
  }

  res.writeHead(200, { "Content-Type": "text/plain" });
  res.end(`Hello from sample-app v${VERSION}! (pid=${process.pid})\n`);
});

server.listen(PORT, () => {
  console.log(`sample-app listening on :${PORT}`);
});

process.on("SIGTERM", () => {
  server.close(() => process.exit(0));
});
