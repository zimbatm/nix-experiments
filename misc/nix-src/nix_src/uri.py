import re



def parse(uri, args=dict()):
    '''
    parse("channel:nixos-18.03", dict())
    parse("nixos/nixpkgs-channels", dict(branch="nixos-18.03"))
    parse("https://github.com/nixos/nixpkgs-channels/archive/nixos-18.03.tar.gz")
    '''

