#!/usr/bin/env python
# -*- coding: utf-8 -*-

from setuptools import find_packages, setup

version = open('version.txt').read().strip()

setup(
    name = 'nix-src',
    version = version,
    description = 'nix source fetcher and pinning tool',
    author = 'zimbatm',
    author_email = 'zimbatm@zimbatm.com',
    url = 'https://github.com/nix-community/nix-src',
    packages = find_packages(exclude=['tests']),
    entry_points = {
        'console_scripts': [
            'nix-src = nix_src.__main__:main'],
    },
    install_requires = [
        'docopt>=0.6.0'
    ],
    include_package_data = True,
    license = 'ISC'
)
