# How would it look like if you could generate HTML5 with Nix?
#
# This is like https://rubygems.org/gems/markaby
with builtins;
rec {
  tag = type:
    let
      elem = { _type = "tag"; type = type; };
    in
    # either an attribute set or a null.
    # if null is passed, the 3rd argument here is ignored. Useful for <br/> or
    # <hr/>.
    attrs:
    if attrs == null then elem // { attrs = {}; body = []; } else
    # either a string or list of (element or string)
    body:
    elem // {
      attrs = attrs;
      body = if builtins.typeOf body == "string" then [ body ] else body;
    };


  # TODO: add <DOCTYPE> in front.
  html5 = tag "html";
  head = tag "head";
  body = tag "body";
  meta = tag "meta";
  script = tag "script";
  div = tag "div";
  hr = tag "hr";
  a = tag "a";
  # ... add all the tags here

  # TODO:
  renderHTML = { depth ? 0, elems }: "TODO";

  example =
    html5 { lang = "en"; } [
      (head {} [
        (meta { lang = "utf-8"; } [])
        (script { src = "https://xxx"; } [])
      ])
      (body {} [
        (div {} [
          "Hey this is a text area"
          (hr null)
          "And another"
          (a { href = https://example.com; } "Example website")
        ])
      ])
    ];


}
