{ ... }:
rec {
  isType = builtins.isString;
  toType = builtins.toString;

  sub = builtins.substring;

  append = a: b: a + b;

  # [string] -> string
  concat = concatSep "";

  # string -> [string] -> string
  concatSep = builtins.concatStringsSep;

  #
  length = builtins.stringLength;

  optional = cond: x: if cond then x else empty;

  empty = "";

  isEmpty = x: x == empty;

  # start:int -> length:int -> [T] -> [T]
  slice = builtins.substring;

  split = builtins.split;

  take = count: slice 0 count;

  drop = count: str: slice count (length str) str;

  replace = builtins.replaceStrings;

  # Returns a hash of the content.
  #
  # Valid algos are:
  #   md5
  #   sha1
  #   sha256
  #   sha512
  #
  # algo:string -> content:string -> hash:string
  hash = builtins.hashString;

  # regexp match
  match = builtins.match;
}
