# Usage

For now all that we have a is a Ubuntu VM to test the installer manually.
Automated tests come next.

Run ./wootbuntu to start the VM. It will automatically download the ISO and
setup QEMU. Make sure to have KVM enabled on your machine for it to be fast
(/dev/kvm should exist on the host).

# Login

Next comes the login session. Use these credentials:

username: ubuntu
password: ubuntu

# SSH access

From the installer-test folder, run:

```sh
./ssh
```

# FIXME: mount /mnt

```sh
mount -t 9p hostshare /mnt -o trans=virtio,version=9p2000.L
```

# Run the installer

The nix folder is mounted read-only under /mnt. Build the installer on the
host and then run the installer on the guest.

# Shutdown

Press `ctrl+a c` to open the qemu console and then type `quit`.

[1]: https://bugs.launchpad.net/cloud-images/+bug/1726476
