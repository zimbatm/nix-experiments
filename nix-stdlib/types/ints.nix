{ ... }:
{
  isType = builtins.isInt;

  even = a: (mod a 2) == 0;
  odd = a: (mode a 2) != 0;

  lessThan = builtins.lessThan;

  min = x: y: if x < y then x else y;
  max = x: y: if x > y then x else y;

  add = builtins.add;
  sub = builtins.sub;
  mul = builtins.mul;
  div = builtins.div;
}
