{ lists, ... }:
rec {
  isType = builtins.isAttrs;

  concat = a: b: a // b;

  size = set: builtins.length (keys set);

  optional = cond: x: if cond then x else empty;

  empty = {};

  isEmpty = x: x == empty;

  get = builtins.getAttr;

  has = builtins.hasAttr;

  map = builtins.mapAttr;

  remove = builtins.remoteAttrs;

  keys = builtins.attrNames;

  values = builtins.attrValues;

  # key:string -> [{ key = value; }] -> [value]
  cat = builtins.catAttrs;

  intersect = builtins.intersectAttrs;

  # filter = pred: set:
  # lists.toAttrs
  # (lists.concatMap
  # (name: let v = set.${name}; in if pred name v then [(nameValuePair name v)] else []) (keys set));

}
