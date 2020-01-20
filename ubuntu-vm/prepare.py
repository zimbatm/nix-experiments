#!/usr/bin/env python
#
# Usage: ./prepare.py <qemu-runner> [...<args>]

import pexpect
import shlex
import subprocess
import sys

def log(msg):
    print("[prepare] " + msg)

# should really be shlex.join which is available in python 3.8
def shlex_join(args):
    " ".join(args)

log("args={}".format(sys.argv[1:]))

log("booting VM")

qemu = pexpect.spawn(
        sys.argv[1],
        sys.argv[2:],
        logfile = sys.stdout,
        encoding = 'utf8',
        timeout = 1000,
        )

# work around a bug in the image
qemu.expect(u"error: no such device: root.")
qemu.sendline("")

log("waiting on boot to finish")

qemu.expect(u"cloud-init.*finished at ")

log("logging in")

qemu.sendline("ubuntu")
qemu.expect(u"Password:")
qemu.sendline("ubuntu")
qemu.expect(u"ubuntu@ubuntu")

log("entering qemu menu")

qemu.sendcontrol("a")
qemu.send("c")

log("creating snapshot")

qemu.expect(u"\(qemu\)")
qemu.sendline("savevm prepare")

log("exiting")

qemu.expect(u"\(qemu\)")
qemu.sendline("quit")
qemu.wait()

log("FINISHED")
