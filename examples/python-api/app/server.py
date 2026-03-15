"""Minimal Python HTTP API using only the standard library."""

import json
import os
import urllib.request
from http.server import HTTPServer, BaseHTTPRequestHandler


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/":
            self._text(200, "Hello from chiseled Python!\n")
        elif self.path.startswith("/fetch"):
            self._handle_fetch()
        elif self.path == "/healthz":
            self._text(200, "ok\n")
        else:
            self._text(404, "not found\n")

    def _handle_fetch(self):
        url = "https://api.github.com/zen"
        # Allow ?url= override
        if "?" in self.path:
            qs = self.path.split("?", 1)[1]
            for part in qs.split("&"):
                if part.startswith("url="):
                    url = part[4:]
        try:
            with urllib.request.urlopen(url) as resp:
                body = resp.read()
            self._text(200, body.decode("utf-8", errors="replace"))
        except Exception as e:
            self._text(502, f"fetch error: {e}\n")

    def _text(self, code, body):
        self.send_response(code)
        self.send_header("Content-Type", "text/plain")
        self.end_headers()
        self.wfile.write(body.encode())

    def log_message(self, fmt, *args):
        print(f"[server] {fmt % args}")


def main():
    port = int(os.environ.get("PORT", "8080"))
    server = HTTPServer(("", port), Handler)
    print(f"listening on :{port}")
    server.serve_forever()


if __name__ == "__main__":
    main()
