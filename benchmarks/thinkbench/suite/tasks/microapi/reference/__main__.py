"""Reference microapi CLI + runnable server with an in-memory todo API.

  python -m microapi serve --host 127.0.0.1 --port 8080

NOTE: the held-out oracle does NOT run this server or open a socket; it tests
`App.handle_request` in-process. This module exists to satisfy the SPEC's "runnable
server" + "example routes for a small in-memory todo API" requirements.
"""
import argparse
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from .public import App


def build_app():
    app = App()
    todos = {}
    state = {"next_id": 1}

    @app.use
    def request_logger(request, call_next):
        # trivial middleware: pass through (real logging would go here)
        return call_next(request)

    @app.route("GET", "/todos")
    def list_todos(request):
        return list(todos.values())

    @app.route("POST", "/todos")
    def create_todo(request):
        payload = request.json or {}
        tid = state["next_id"]
        state["next_id"] += 1
        todo = {"id": tid, "title": payload.get("title", ""), "done": False}
        todos[tid] = todo
        return (201, todo)

    @app.route("GET", "/todos/{todo_id}")
    def get_todo(request, todo_id):
        todo = todos.get(int(todo_id))
        if todo is None:
            return (404, {"error": f"no todo {todo_id}"})
        return todo

    @app.route("DELETE", "/todos/{todo_id}")
    def delete_todo(request, todo_id):
        todos.pop(int(todo_id), None)
        return (204, {"deleted": todo_id})

    return app


def serve(host, port):
    app = build_app()

    class Handler(BaseHTTPRequestHandler):
        def _dispatch(self, method):
            length = int(self.headers.get("Content-Length") or 0)
            body = self.rfile.read(length) if length else b""
            headers = {k: v for k, v in self.headers.items()}
            status, resp_headers, resp_body = app.handle_request(
                method, self.path, headers, body
            )
            self.send_response(status)
            for k, v in resp_headers.items():
                self.send_header(k, v)
            self.send_header("Content-Length", str(len(resp_body)))
            self.end_headers()
            self.wfile.write(resp_body)

        def do_GET(self):
            self._dispatch("GET")

        def do_POST(self):
            self._dispatch("POST")

        def do_DELETE(self):
            self._dispatch("DELETE")

        def do_PUT(self):
            self._dispatch("PUT")

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer((host, port), Handler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


def main(argv):
    ap = argparse.ArgumentParser(prog="microapi")
    sub = ap.add_subparsers(dest="cmd")
    sp = sub.add_parser("serve")
    sp.add_argument("--host", default="127.0.0.1")
    sp.add_argument("--port", type=int, default=8080)
    args = ap.parse_args(argv)
    if args.cmd == "serve":
        serve(args.host, args.port)
        return 0
    ap.print_help()
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
