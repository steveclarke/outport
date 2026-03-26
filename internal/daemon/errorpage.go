package daemon

import (
	"fmt"
	"html"
	"net/http"
)

const errorPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
<style>
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
@keyframes fadeIn{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:translateY(0)}}
@keyframes breathe{0%%,100%%{opacity:.12}50%%{opacity:.2}}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',system-ui,sans-serif;min-height:100vh;display:flex;align-items:center;justify-content:center;background:#fafbfc;padding:2rem}
.c{text-align:center;max-width:420px;animation:fadeIn .4s ease-out}
.mark{width:48px;height:48px;margin:0 auto 1.5rem;animation:breathe 4s ease-in-out infinite}
.host{font-family:'SF Mono','Fira Code',monospace;font-size:1.125rem;color:#1e293b;font-weight:600;margin-bottom:.5rem}
.msg{font-size:.9375rem;color:#64748b;line-height:1.6;margin-bottom:1.5rem}
.msg code{font-family:'SF Mono','Fira Code',monospace;font-size:.875rem;color:#475569;background:#f1f5f9;padding:.125rem .375rem;border-radius:4px}
.hint{display:inline-block;font-family:'SF Mono','Fira Code',monospace;font-size:.8125rem;color:#475569;background:#f1f5f9;padding:.375rem .75rem;border-radius:6px;border:1px solid #e2e8f0}
.foot{margin-top:2rem;font-size:.75rem;color:#94a3b8}
.foot a{color:#64748b;text-decoration:none;transition:color .2s}
.foot a:hover{color:#475569}
@media(prefers-reduced-motion:reduce){.c{animation:none}.mark{animation:none;opacity:.15}}
</style>
</head>
<body>
<div class="c">
<svg class="mark" viewBox="0 0 454 428" xmlns="http://www.w3.org/2000/svg">
<path fill="#031c54" d="m216.4.3c4-.4 9.1-.3 13.2-.3 53.4-.3 107.3 20.7 140.3 63.9 19.9 25.9 31.2 55.9 33.7 88.6 1.1 13.9.9 27.2.8 41.1-8.9-4.1-18.4-7-28.1-8.4-29-4.3-51.8 3.3-74.9 20.2l.1-34.4c0-27.2 0-50.4-17.1-73.9-11-15.1-27-25-45.6-27.9-4.2-.6-8.5-.8-12.7-.8-40.9.1-65.1 30.2-70.7 68.6-1.7 11.5-1.5 22-1.5 33.6 0 11.6.1 23.2.3 34.8-8.2-6.6-17.5-11.8-27.3-15.4-25.6-9.4-52.1-7.6-76.7 3.7.3-13.5-.1-26.7.8-40.2C54.9 96.2 88.8 44.8 140.5 19.2 165.2 7 189 2.1 216.4.3z"/>
<path fill="#2E86AB" d="m355.3 198.3c8.9-.6 13.5.4 22.2 2.3-29.9 15.1-47.9 45-51.7 77.9-1.2 10.4-.5 21-.8 31.5-1.7 50.4-39.2 98.8-92.1 100.2-27.6.7-46.6-7-66.7-25.8-11.1-11.2-23.9-29.1-27.2-44.8 1.3 1.3 2.7 2.5 4.1 3.7 16.3 13.8 37.3 20.5 58.5 18.7 12.8-1.2 25.7-6.4 36.1-13.8 16.8-11.7 28.1-29.9 31.2-50.2 1.3-8.8 1.2-16.1 3.2-25.3 3.3-15.4 10.3-29.8 20.5-41.9 16.1-19.2 37.6-30.6 62.6-32.7z"/>
<path fill="#031c54" d="m16.7 199.8c8.7-.5 17.2-.1 25.7 1.8 37.2 8.2 65.8 38 72.6 75.4 2.2 11.5 1.7 20.5 2.1 31.8.6 23.5 7.8 46.5 20.6 66.2 19.8 30.7 46.6 44.9 81.3 52.3-5.3.3-10.9.2-16.3.2-74.2-2.3-134.6-49.5-145.3-125.2-3.4-24.2 2.3-44.8-11.7-67.4-12-19.4-24.2-27.1-45.7-32.8 5.5-1.2 11.1-1.8 16.7-2.2z"/>
<path fill="#031c54" d="m426.5 199.8c8.5-.8 19.1.7 27.4 2.2-33.8 8.3-50.7 31.7-54.6 65.9-.5 5.9-.4 11.9-.5 17.8-1.3 75.5-59.5 134.3-134 140.9-8 1-20.4.9-28.4.5 59.3-10.6 96.6-54.6 101.4-114.2.6-8 .4-16 .9-24 1.3-21.4 9.6-41.7 23.7-57.7 16.6-19.2 38.9-29.6 64.1-31.4z"/>
<path fill="#2E86AB" d="m89.9 198.8c42-3.5 81.4 28 93.5 67.1 3.1 10.1 3.7 20.2 8.8 29.9 6.1 11.5 15.6 21.5 28.3 25.7 12.5 4.1 22.6-1.5 33.4-7.1-5.3 7.7-10.2 13.8-17.6 19.6-34.5 25.1-86.3 10.4-102.9-28.9-3-6.6-2.5-13.6-2.9-20.7-1.7-32.4-22.3-70.9-53.4-83.8 4.8-1 8-1.4 12.8-1.8z"/>
</svg>
<div class="host">%s</div>
<div class="msg">%s</div>
%s
<div class="foot">Routed by <a href="https://outport.dev">Outport</a></div>
</div>
</body>
</html>`

func writeErrorPage(w http.ResponseWriter, status int, hostname, message, hint string) {
	safeHost := html.EscapeString(hostname)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, errorPageTemplate, safeHost+" — Outport", safeHost, message, hint)
}
