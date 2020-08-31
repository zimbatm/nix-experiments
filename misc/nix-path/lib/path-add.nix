{ path
, name
, url
, ...
}@args:
(import path) // { "${name}" = builtins.removeAttrs args [ "path" "name" ]; }
