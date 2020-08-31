#!/usr/bin/env python
# -*- coding: utf-8 -*-

import sys

def main():
    try:
        from .cli import main
        sys.exit(main())
    except KeyboardInterrupt:
        from . import ExitStatus
        sys.exit(ExitStatus.ERROR_CTRL_C)


if __name__ == '__main__':
    main()
