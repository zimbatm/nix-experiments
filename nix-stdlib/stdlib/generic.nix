{ ... }@stdlib:
rec {
  inherit (builtins) typeOf;

  optional = cond: x:
    let t = typeOf x; in
    if cond then x else stdlib."${t}s".empty;

  # assuming a and b are of the same type
  append = a: b:
    let t = typeOf a; tb = typeOf b; in assert t == tb;
    stdlib."${t}s".append a b;
}
