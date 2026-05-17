# Swarm mDNS Publisher

Publishes mDNS/DNS-SD records for Docker Swarm services based on service labels.

The service watches the local Docker Engine API through `/var/run/docker.sock`,
selects Swarm services with `mdns.enable=true`, and advertises ingress-published
ports with `brutella/dnssd`.

## Labels

- `mdns.enable=true`
- `mdns.hostname=portainer.local`
- `mdns.address=10.45.45.2`
- `mdns.service.type=_https._tcp`
- `mdns.service.name=Portainer`

Only ingress-published ports are advertised by default.

## Image

CI publishes the container image to:

```text
ghcr.io/aadnehovda/swarm-mdns-publisher:latest
```

Tagged releases also publish a matching semver tag.
