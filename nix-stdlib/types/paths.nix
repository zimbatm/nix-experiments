{ ... }:
{
  isType = builtins.isPath;

  # FIXME: is this correct?
  toType = str: /. + toString str;

  exists = builtins.pathExists;

  # path -> string
  read = builtins.readFile;

  # creates a new entry in the /nix/store
  #
  # name:string -> content:string -> path
  write = builtins.toFile;

  # path -> set{ name = filetype }
  readDir = builtins.readDir;

  dirname = builtins.dirOf;

  basename = builtins.baseNameOf;

  # resolves a given path to a /nix/store entry
  #
  # path -> string
  storePath = builtins.storePath;

  import = builtins.import;

  # builtins.filterSource

  # builtins.path

  # Find a file in the Nix search path. Used to implement <x> paths,
  # which are desugared to 'findFile __nixPath "x"'. */
  findFile = builtins.findFile;

  # Returns a hash of the file.
  #
  # Valid algos are:
  #   md5
  #   sha1
  #   sha256
  #   sha512
  #
  # algo:string -> path -> hash:string
  hash = builtins.hashFile;
}
