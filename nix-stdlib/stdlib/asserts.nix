{ ... }:
{
  isEqual = a: b:
    if a != b then throw "expected ${a} == ${b}" else true;

  isTrue = cond:
    if !cond then throw "expected true, got ${cond}" else true;
}
