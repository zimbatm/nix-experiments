let
  fetch = import ./.;
in
{
  example1 = fetch {
    url_template = "<homepage>/archive/<rev>.tar.gz";
    hash = "sha256-nlN43/AvaFXpQuuSVYyNJ59d15iorGqzcc9/Hop0FbA=";
    unpack = true;
    meta.rev = "8634c3b619909db7fc747faf8c03592986626e21";
    meta.homepage = "https://github.com/NixOS/nixpkgs-channels";
  };

  example2 = fetch {
    url = "github:direnv/direnv?rev=1.2.3";
    hash = "sha256-nlN43/AvaFXpQuuSVYyNJ59d15iorGqzcc9/Hop0FbA=";
  };
}
