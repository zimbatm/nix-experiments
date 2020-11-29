{ stdenv, runCommand }:
{ nixSrc, nixAttr, drv ? null, name ? nixAttr, binName ? nixAttr }:
runCommand name
{
  passthru = {
    inherit drv;
  };
} ''
  mkdir -p $out/bin
  cat <<'LAZY_BIN' > $out/bin/${binName}
  #! ${stdenv.shell}
  set -euo pipefail
  out=$(nix-build ${nixSrc} -A ${nixAttr} --no-out-link)
  exec "$out/bin/${binName}" "$@"
  LAZY_BIN
  chmod +x $out/bin/${binName}
''
