# devenv-light

Everybody should stop using `nix-shell` and `nix develop`. Without going too
much over the arguments, those are designed to debug the build of a
derivation. The command should actually be renamed to `nix debug`. And also
drop you into the derivation build environment sandbox instead.

Both devshell and devenv tried using the mkNakedShell approach, and that
causes all sorts of problems. But we don't need all of that.

So now. How do we create a clean developer environment, that can reload on
change?

The premise of this POC is that all we need is a little entrypoint script,
that is invokable using `nix run`.

See the `example/` folder for some usage.
