# nix-service - networked service definition

I see largely two classes services in NixOS:

1. system services (eg: upower, polkit, systemd-journald)
2. networked services (eg: nginx, postgresql)

Where (2) is a subset of (1) that is generally closer to a docker container.
Typically it has the following characteristics:

1. It doesn't need or should have access to the system.
2. It binds to one or more ports.
3. It might have a state directory.
4. You might want multiple instances of it.
5. Something else?

Right now, each new service is adding to the global NixOS module namespace,
making the evaluation slower with each new release. If we could define that
subset and make it pluggable, it could allow trimming the list of NixOS
services.

Another benefit is that the subset might be portable to other systems, like
home-manager, devenv, ...

Can we find a subset that works for most of the cases?

## Deployment

I see two ways this can play out:
1. You want to deploy everything at once. In that scenario, the service
   definitions get attached to NixOS and get deployed into the same profile.
2. Each service definition gets its own profile, that can be updated and
   rolled back independently. This also means that we need some sort of
   profile deploying tool.

## TODO

How do we handle ingress for HTTP services? I don't have a good vision for
that right now.

## Prior art

* https://github.com/NixOS/rfcs/pull/163
* https://github.com/svanderburg/dysnomia

