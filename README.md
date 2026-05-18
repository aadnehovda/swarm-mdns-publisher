# Swarm mDNS Publisher

Publishes mDNS/DNS-SD records for Docker Swarm services based on service labels.

The service watches the local Docker Engine API through `/var/run/docker.sock`,
selects Swarm services with `mdns.enable=true`, and advertises ingress-published
ports with `brutella/dnssd`.

## Labels

- `mdns.enable=true`
- `mdns.hostname=portainer.local`
- `mdns.address=10.45.45.2` optional per-service override
- `mdns.service.type=_https._tcp`
- `mdns.service.name=Portainer` optional, defaults to the Docker service name

Only ingress-published ports are advertised by default.

## Address Selection

Ingress-published addresses are selected in this order:

1. Service label `mdns.address`
2. Environment variable `MDNS_DEFAULT_ADDRESS`
3. Automatic local source address detection using UDP probe `MDNS_PROBE_ADDRESS`

Host-published ports are advertised only from the node running the task.
Host mode intentionally ignores `MDNS_DEFAULT_ADDRESS`, because that value may
be an ingress VIP. Host-published addresses are selected from:

1. Service label `mdns.address`
2. Automatic local source address detection using UDP probe `MDNS_PROBE_ADDRESS`

`MDNS_PROBE_ADDRESS` defaults to `224.0.0.251:5353`, the IPv4 mDNS multicast
address. The probe opens a UDP socket and reads the local source address chosen
by the kernel; it does not need to send packets.

## Custom TXT

Labels with the `mdns.txt.` prefix are published as DNS-SD TXT metadata:

```yaml
deploy:
  labels:
    mdns.txt.version: "2026.5.1"
    mdns.txt.internal_url: "http://homeassistant.local:8123"
```

Custom TXT keys cannot override publisher-owned keys: `service`, `stack`,
`protocol`, `target_port`, `published_port`, `publish_mode`, or
`address_source`.

## Examples

### Home Assistant on Ingress

```yaml
services:
  homeassistant:
    image: ghcr.io/home-assistant/home-assistant:stable
    ports:
      - target: 8123
        published: 8123
        protocol: tcp
        mode: ingress
    deploy:
      labels:
        mdns.enable: "true"
        mdns.hostname: "homeassistant.local"
        mdns.service.type: "_home-assistant._tcp"
        mdns.service.name: "Home Assistant"
        mdns.txt.version: "2026.5.1"
        mdns.txt.base_url: "http://homeassistant.local:8123"
        mdns.txt.internal_url: "http://homeassistant.local:8123"
        mdns.txt.requires_api_password: "True"
```

With `MDNS_DEFAULT_ADDRESS=10.45.45.2`, this advertises Home Assistant at
`homeassistant.local:8123` on `10.45.45.2`.

### Host-Published Service

```yaml
services:
  admin:
    image: nginx:alpine
    ports:
      - target: 80
        published: 8080
        protocol: tcp
        mode: host
    deploy:
      replicas: 1
      labels:
        mdns.enable: "true"
        mdns.hostname: "admin.local"
        mdns.service.type: "_http._tcp"
        mdns.service.name: "Admin"
```

This is advertised only by the publisher instance running on the node where the
task is running.

## Image

CI publishes the container image to:

```text
ghcr.io/aadnehovda/swarm-mdns-publisher:latest
```

Tagged releases also publish a matching semver tag.
