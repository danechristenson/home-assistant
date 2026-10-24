// Home Assistant's "Open web UI" (ingress): sends the browser on to the
// address players use. An add-on's own webui link can only be Home
// Assistant's address and a port, never a hostname from the options.
// FOUNDRY_URL is set by run.sh; empty means Home Assistant's address, port 30000.
import http from "node:http";

// Only Home Assistant's ingress proxy, and the smoke test from inside
const ALLOWED = new Set(["172.30.32.2", "127.0.0.1", "::1"]);

const url = JSON.stringify(process.env.FOUNDRY_URL || "").replace(/</g, "\\u003c");
const page = `<!doctype html>
<meta charset="utf-8">
<title>Foundry VTT</title>
<p><a id="link" target="_top" href="#">Open Foundry VTT</a></p>
<script>
  const url = ${url} || "http://" + location.hostname + ":30000/";
  document.getElementById("link").href = url;
  window.top.location.href = url;
</script>
`;

http.createServer((request, response) => {
    const address = (request.socket.remoteAddress || "").replace(/^::ffff:/, "");
    if (!ALLOWED.has(address)) {
        response.writeHead(403).end();
        return;
    }
    response.writeHead(200, { "Content-Type": "text/html; charset=utf-8", "Cache-Control": "no-store" });
    response.end(page);
}).listen(8099);
