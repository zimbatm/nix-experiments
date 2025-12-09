# Script to run the test UKI in QEMU
#
# This is a thin wrapper around run-installer-vm that uses the test-uki
# instead of the installer-uki.
{
  perSystem,
  ...
}:
perSystem.self.run-installer-vm.override {
  uki = perSystem.self.test-uki;
  name = "run-test-vm";
}
