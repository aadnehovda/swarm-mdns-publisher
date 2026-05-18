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

Advertised addresses are selected in this order:

1. Service label `mdns.address`
2. Environment variable `MDNS_DEFAULT_ADDRESS`
3. Automatic local source address detection using UDP probe `MDNS_PROBE_ADDRESS`

`MDNS_PROBE_ADDRESS` defaults to `224.0.0.251:5353`, the IPv4 mDNS multicast
address. The probe opens a UDP socket and reads the local source address chosen
by the kernel; it does not need to send packets.

## Image

CI publishes the container image to:

```text
ghcr.io/aadnehovda/swarm-mdns-publisher:latest
```

Tagged releases also publish a matching semver tag.
