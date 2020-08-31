{ fetchers }:
srcJSON:
let
  importJSON = file: builtins.fromJSON (builtins.readFile file);
  attrs = importJSON srcJSON;
  fetcher = fetchers.${"fetch" + attrs.fetcher} or (throw "Unknown fetcher ${attrs.fetcher}");
in
fetcher attrs.src
