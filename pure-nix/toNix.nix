let
  inherit (builtins)
    attrNames
    concatStringsSep
    isAttrs
    isInt
    isList
    isPath
    isString
    replaceStrings
    typeOf
    ;

  # Copied from <nixpkgs/lib>
  mapAttrsToList = f: attrs:
    map (name: f name attrs.${name}) (attrNames attrs);

  # Escape Nix strings
  stringEscape = str:
    "\"" + (
      replaceStrings
        [ "\\" "\"" "\n" "\r" "\t" ]
        [ "\\\\" "\\" "\\n" "\\r" "\\t" ]
        str
    )
    + "\"";

  # Like builtins.JSON but to output Nix code
  toNix = value:
    if isString value then stringEscape value
    else if isInt value then toString value
    else if isPath value then toString value
    else if true == value then "true"
    else if false == value then "false"
    else if null == value then "null"
    else if isAttrs value then
      "{ " + concatStringsSep " " (mapAttrsToList (k: v: "${k} = ${toNix v};") value) + " }"
    else if isList value then
      "[ ${ concatStringsSep " " (map toNix value) } ]"
    else throw "type ${typeOf value} not supported";
in
toNix
