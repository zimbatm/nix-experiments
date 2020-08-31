# nix-dev - a holistic approach to nix-based development

Have you ever struggled with development environments?

All the tests are passing for your colleague, but not on your machine. It
turns out that libxml has the be installed and it was not declared anywhere.

It takes ages to setup he CI.

## Getting started

Install mkdev:

```sh
$ eval "$(curl -sfL https://mkdev.dev/install.sh)"
Installing mkdev...

nix not found, installing nix 2.2.1

shell detected: bash, installing profile...

Installation finished.
```


### Project scaffolding

Then in your project run `scaffold` to install the project scaffolding.

```
$ cd path/to/project
# create a scaffold with Travis CI configuration:
$ scaffold --layout zimbatm/default
Fetching scaffolding from zimbatm/default

  Installing default.nix...
  
  Adding fmt.sh...
  
  Adding env.sh...

[env] No cache detected, loading the environment...
$ # ready to develop
```

### Managing nix sources

TODO: replaced by the nix flakes

```
$ nix-src add foo/bar
```

```
$ nix-src path # Print the nix-path
```

### Formatting code

Really fast

```
$ fmt
```

List all of the available formatters:
```
$ fmt list
```

### Project environment

```
$ ./env.sh ls
```

### Project navigation

```
$ h foo/bar
```

### Language servers


### CI setup

```
$ dev ci
```

### Deploy

```
$ depl --target production
```

### Workbook

```
$ mdsh
```

