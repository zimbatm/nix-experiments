{
  description = "Image builder for nixpkgs commit";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};

      # What to use as the root of our disk images
      rootFs = nixpkgs;

      # Hacky version with no escaping
      cmd = args: nixpkgs.lib.concatStringsSep " " args;

      erofs =
        pkgs.runCommand "nixpkgs-source.erofs"
          {
            buildInputs = [ pkgs.erofs-utils ];
          }
          (cmd [
            "mkfs.erofs"
            "--all-root" # make all files owned by root
            "--chunksize=4096" # larger blocks are easier to compress
            "-zzstd,level=22"
            "$out"
            rootFs
          ]);

      squashfs =
        pkgs.runCommand "nixpkgs-source.squashfs"
          {
            buildInputs = [ pkgs.squashfsTools ];
          }
          (cmd [
            "mksquashfs"
            rootFs
            "$out"
            "-noappend"
            "-all-root" # makes all files owned by root
            "-no-xattrs"
            "-b 1M" # bigger block sizes compress better
            "-comp zstd"
            "-Xcompression-level 22"
          ]);

      tarball =
        pkgs.runCommand "nixpkgs-source.tar.zstd"
          {
            nativeBuildInputs = [ pkgs.zstd ];
          }
          ''
            tar --sort=name --owner=0 --group=0 --numeric-owner -cf - ${nixpkgs} | zstd -22 -T0 -o $out
          '';

      report = pkgs.runCommand "size-report" { } ''
        cat <<EOF | tee $out
        source = $(du -sh ${nixpkgs})
        erofs = $(du -sh ${erofs})
        squashfs = $(du -sh ${squashfs})
        tarball = $(du -sh ${tarball})
        EOF
      '';
    in
    {
      packages.${system} = {
        inherit
          erofs
          squashfs
          tarball
          report
          ;
      };
    };
}
