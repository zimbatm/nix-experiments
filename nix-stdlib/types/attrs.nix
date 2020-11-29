{ lists, ... }:
rec {
  isType = builtins.isAttrs;

  concat = a: b: a // b;

  size = set: builtins.length (keys set);

  optional = cond: x: if cond then x else empty;

  empty = { };

  isEmpty = x: x == empty;

  get = builtins.getAttr;

  has = builtins.hasAttr;

  map = builtins.mapAttrs;

  remove = builtins.removeAttrs;

  keys = builtins.attrNames;

  values = builtins.attrValues;

  # key:string -> [{ key = value; }] -> [value]
  cat = builtins.catAttrs;

  intersect = builtins.intersectAttrs;

  # filter = pred: set:
  # lists.toAttrs
  # (lists.concatMap
  # (name: let v = set.${name}; in if pred name v then [(nameValuePair name v)] else []) (keys set));

  /* Call a function for each attribute in the given set and return
     the result in a list.

     Example:
       mapAttrsToList (name: value: name + value)
          { x = "a"; y = "b"; }
       => [ "xa" "yb" ]
  */
  mapToList = f: attrs:
    builtins.map (name: f name attrs.${name}) (keys attrs);
}
