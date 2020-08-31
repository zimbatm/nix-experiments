source "$(bats-helpers)"

setup() {
  HELLO=1
}

teardown() {
  :
}

@test "./env.sh --help" {
  run ./pkgs/devenv/share/devenv/templates/default/env.sh --help
  assert_output --partial "Usage:"
}

@test "./env.sh echo OK" {
  run ./pkgs/devenv/share/devenv/templates/default/env.sh echo OK
  assert_output --partial "OK"
}
