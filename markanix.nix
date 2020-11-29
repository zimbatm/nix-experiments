# How would it look like if you could generate HTML5 with Nix?
#
# This is like https://rubygems.org/gems/markaby
rec {
  # TODO: escape values if necessary
  renderAttrs = attrs:
    if attrs == { } then ""
    else
      " " +
      (builtins.concatStringsSep " "
        (map (k: "${k}=${toString attrs.${k}}") (builtins.attrNames attrs)));

  tag = type: attrs: body:
    {
      tag = type;
      attrs = attrs;
      body = if builtins.isString body then [ body ] else body;
      __toString = self:
        "<${self.tag}${renderAttrs self.attrs}>"
        + (builtins.concatStringsSep "\n" (map toString self.body))
        + "</${self.tag}>"
      ;
    };

  # tag with no body
  nulTag = type: attrs:
    {
      tag = type;
      attrs = attrs;
      __toString = self:
        "<${self.tag}${renderAttrs self.attrs}>";
    };


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

  exampleRendered = toString example;
}
