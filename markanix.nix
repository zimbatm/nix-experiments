# How would it look like if you could generate HTML5 with Nix?
#
# This is like https://rubygems.org/gems/markaby
rec {
  tag = type:
    let
      elem = { tag = type; };
    in
    # either an attribute set or a null.
      # if null is passed, the 3rd argument here is ignored. Useful for <br/> or
      # <hr/>.
    attrs:
    if attrs == null then elem // { attrs = { }; body = [ ]; } else
      # either a string or list of (element or string)
    body:
    elem // {
      attrs = attrs;
      body = if builtins.isString body then [ body ] else body;
    };

  nulTag = type: attrs: tag type attrs null;

  # TODO: add <DOCTYPE> in front.
  html5 = tag "html";
  head = tag "head";
  body = tag "body";
  meta = nulTag "meta";
  script = tag "script";
  div = tag "div";
  hr = nulTag "hr";
  a = tag "a";
  # ... add all the tags here

  # TODO: escape values if necessary
  renderAttrs = attrs:
    if attrs == { } then ""
    else
      " " +
      (builtins.concatStringsSep " "
        (map (k: "${k}=${toString attrs.${k}}") (builtins.attrNames attrs)));

  renderHTML = tree:
    if builtins.isAttrs tree then
      "<${tree.tag}${renderAttrs tree.attrs}>"
      +
      (if tree.body == null then ""
      else
        (builtins.concatStringsSep "\n" (map renderHTML tree.body))
        + "</${tree.tag}>")
    else
      toString tree
  ;

  example =
    html5 { lang = "en"; } [
      (head { } [
        (meta { lang = "utf-8"; })
        (script { src = "https://xxx"; } [ ])
      ])
      (body { } [
        (div { } [
          "Hey this is a text area"
          (hr { })
          "And another"
          (a { href = https://example.com; } "Example website")
        ])
      ])
    ];

  exampleRendered = renderHTML example;
}
