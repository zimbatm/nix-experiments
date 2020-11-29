let
  # Copied from <nixpkgs/lib>
  mapAttrsToList = f: attrs:
    map (name: f name attrs.${name}) (builtins.attrNames attrs);

  # Escape Nix strings
  stringEscape = str:
    "\"" + (
      builtins.replaceStrings
        [ "\\" "\"" "\n" "\r" "\t" ]
        [ "\\\\" "\\" "\\n" "\\r" "\\t" ]
        str
    )
    + "\"";

  # Like builtins.JSON but to output Nix code
  toNix = value:
    if builtins.isString value then stringEscape value
    else if builtins.isInt value then toString value
    else if builtins.isPath value then toString value
    else if true == value then "true"
    else if false == value then "false"
    else if null == value then "null"
    else if builtins.isAttrs value then
      "{ " + builtins.concatStringsSep " " (mapAttrsToList (k: v: "${k} = ${toNix v};") value) + " }"
    else if builtins.isList value then
      "[ ${ builtins.concatStringsSep " " (map toNix value) } ]"
    else throw "type ${builtins.typeOf value} not supported";
in
toNix
