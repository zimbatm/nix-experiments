{ attrs, strings, generic, ... }:
let
  # Escape Nix strings
  stringToNix = str:
    "\"" + (
      strings.replace
        [ "\\" "\"" "\n" "\r" "\t" ]
        [ "\\\\" "\\" "\\n" "\\r" "\\t" ]
        str
    )
    + "\"";

  attrsToNix = value:
    strings.concatSep " " (
      [ "{" ] ++ (attrs.mapToList (k: v: "${k} = ${toNix v};") value)
      ++ [ "}" ]
    );

  listToNix = value:
    strings.concatSep " " (
      [ "[" ] ++ (map toNix value)
      ++ [ "]" ]
    );

  # Like builtins.JSON but outputs Nix code instead
  # TODO:
  # * support floats
  # * escape attrs keys
  # * formatting options?
  toNix = value:
    {
      "bool" = (x: if x then "true" else "false");
      "int" = toString;
      "list" = listToNix;
      "null" = (x: "null");
      "path" = toString;
      "set" = attrsToNix;
      "string" = stringToNix;
    }.${generic.typeOf value} or
      (x: throw "type '${generic.typeOf value}' not supported")
      value
  ;
in
toNix
