---
description: Share local services via Cloudflare tunnels and access them from mobile devices with QR codes.
---

# Sharing & Mobile Access

## Tunneling with outport share

`outport share` tunnels your HTTP services to public URLs via Cloudflare quick tunnels. Requires `cloudflared` (`brew install cloudflared`).

```bash
outport share              # tunnel all HTTP services
outport share web          # tunnel a specific service
```

The command blocks until you press Ctrl+C.

Each hostname gets its own tunnel — if a service has named [aliases](/reference/configuration#aliases), each alias is tunneled independently alongside the primary hostname. All tunnels route through the local proxy so host-based routing works correctly. The maximum number of concurrent tunnels is controlled by the [`tunnels.max` setting](/reference/configuration#global-settings) (default `8`).

While sharing, `.env` files are rewritten so computed values using `${service.url}` resolve to the tunnel URLs automatically. CORS origins, API base URLs, and other cross-service values just work. Values using `${service.url:direct}` stay as localhost (server-to-server calls still go direct). On exit, `.env` files revert to local URLs. Restart your services after starting and stopping `outport share`.

## QR codes for mobile access

`outport qr` generates QR codes so you can open your services on a phone or tablet over the local network.

```bash
outport qr                 # QR codes for all HTTP services (LAN URLs)
outport qr --tunnel        # QR codes using tunnel URLs instead
```

The dashboard at [https://outport.test](https://outport.test) also shows QR codes for each service — tap the QR icon next to any service to scan it.

## Tunnel troubleshooting

### 502 Bad Gateway

The tunnel connected but nothing is listening on the local port. Make sure the service is actually running on the port shown in `outport share` output.

### Vite blocks the tunnel hostname

If you see "Blocked request" and a message about `server.allowedHosts`, add `.trycloudflare.com` to your Vite config:

```ts
// vite.config.ts
server: {
  allowedHosts: [".test", ".trycloudflare.com"]
}
```

### Rails blocks the tunnel hostname

If you see a red "Blocked hosts" error page, Rails is rejecting the `.trycloudflare.com` hostname. Add it to your `config/environments/development.rb`:

```ruby
config.hosts << ".trycloudflare.com"
```

Restart your Rails server after adding this.
