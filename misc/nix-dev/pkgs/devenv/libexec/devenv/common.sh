top=$(cd "${BASH_SOURCE[0]%/*}/../.." && pwd)
# shellcheck disable=SC2034
libexec_dir=$top/share/devenv
# shellcheck disable=SC2034
share_dir=$top/share/devenv

GREEN=
#RED=
OFF=

# set the colors if the stderr is a tty
if [[ -t 2 ]]; then
  GREEN="\033[32;01m"
  #RED="\033[31;01m"
  OFF="\033[0m"
fi

## Functions

# log <args>...
log() {
  echo -e "${GREEN}[devenv]${OFF} $*" >&2
}

script-usage() {
  local line
  read -r _ # ignore the shebang
  while IFS=$'\n' read -r line; do
    if [[ $line != "#"* ]]; then
      break
    fi
    line=${line###}
    line=${line## }
    echo "$line"
  done
}

basename() {
  : "${1%/}"
  printf '%s\n' "${_##*/}"
}

