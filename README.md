# Proof of Concept - Docker domain

This show you basic concept of how to reach docker container from host machine over domain names.

A docker container with name 'mycontainer' in the default network is reachable with domain name `mycontainer.docker`
and a docker container with name `mycontainer` in the network `mynetwork` should be reachable with domain name `mycontainer.mynetwork.docker`.

## My local environment

- Arch Linux
- systemd-resolved

## Configuration

Make sure systemd-resolved is enabled/started.

Configure file `/etc/nsswitch.conf` properly:

```
$ cat /etc/nsswitch.conf | grep ^hosts
hosts: mymachines mdns4_minimal [NOTFOUND=return] resolve [!UNAVAIL=return] files myhostname dns
```

Make sure `resolve [!UNAVAIL=return]` exist before `dns`.

Place a conf file to directory `/etc/systemd/resolved.conf.d/` like following:

```
$ cat /etc/systemd/resolved.conf.d/docker.conf
[Resolve]
DNS=127.0.0.1:5354
DNSSEC=false
Domains=~docker.
```

Start the service with

```
$ docker compose up -d
```

If started with no problem, you can test it with

```
$ resolvectl query dns-server.poc-docker-domain.docker
dns-server.poc-docker-domain.docker: 172.20.0.2

-- Information acquired via protocol DNS in 7.9ms.
-- Data is authenticated: no; Data was acquired via local or encrypted transport: no
-- Data from: network
```

or

```
$ getent hosts dns-server.poc-docker-domain.docker
172.20.0.2      dns-server.poc-docker-domain.docker
```

Note that network lookup commands like `host`, `dig`, `nslookup` and `drill` doesn't work.
because they don't use dns resolver .

## Reference

- [systemd-resolved](https://wiki.archlinux.org/title/systemd-resolved)
- [resolved.conf](https://www.freedesktop.org/software/systemd/man/resolved.conf.html)
- [Domain Name Resolution](https://wiki.archlinux.org/title/Domain_name_resolution)
- [resolvectl](https://www.freedesktop.org/software/systemd/man/resolvectl.html)
- [getent](https://man7.org/linux/man-pages/man1/getent.1.html)