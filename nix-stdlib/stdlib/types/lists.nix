{ ... }:
rec {
  isType = builtins.isList;

  inherit (builtins)
    all
    any
    elem
    elemAt
    map
    filter
    fold'
    genList
    head
    sort
    isList
    length
    tail
    ;

  # [a] -> [a] -> [a]
  append = a: b: a ++ b;

  # [[a]] -> [a]
  concat = builtins.concatLists;

  optional = cond: x: if cond then x else empty;

  empty = [];

  isEmpty = x: x == empty;

  singleton = x: [ x ];

  slice = builtins.sublist;

  take = count: slice 0 count;

  drop = count: list: slice count (length list) list;

  toAtts = builtins.listToAttrs;

  # sort: (a -> a -> bool) -> [a] -> [a]
  sort = builtins.sort;

  replace = builtins.replaceStrings;
}
