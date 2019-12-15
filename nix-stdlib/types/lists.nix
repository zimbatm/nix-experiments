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
    foldl'
    genList
    head
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

  slice = start: count: list:
    let
      len = length list;
    in
      genList
        (n: elemAt list (n + start))
        (
          if start >= len then 0
          else if start + count > len then len - start
          else count
        );

  take = count: slice 0 count;

  drop = count: list: slice count (length list) list;

  toAtts = builtins.listToAttrs;

  # sort: (a -> a -> bool) -> [a] -> [a]
  sort = builtins.sort;

  replace = builtins.replaceStrings;
}
