# Similar to `builtins.toJSON`, output a Nix-formatted string.
#
# Only works on JSON-like values.
#
# TODO: move that into a lib
with builtins;
let
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
