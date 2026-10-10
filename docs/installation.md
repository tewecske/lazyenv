# Installation

## From releases

Download the latest binary for your platform from [GitHub Releases](https://github.com/lazynop/lazyenv/releases). Builds are available for Linux, macOS, Windows, and FreeBSD (amd64 and arm64).

## Homebrew (macOS & Linux)

```bash
brew install lazynop/tap/lazyenv
```

## AUR (Arch Linux)

Install with your preferred AUR helper:

```bash
yay -S lazyenv-bin
```

## Scoop (Windows)

```powershell
scoop bucket add lazynop https://github.com/lazynop/scoop-bucket
scoop install lazyenv
```

## Nix

The repository is a flake. To try it or install it into your profile:

```bash
nix run github:lazynop/lazyenv
nix profile install github:lazynop/lazyenv
```

### NixOS

Add the flake as an input and enable the module:

```nix
{
  inputs.lazyenv.url = "github:lazynop/lazyenv";

  outputs = { nixpkgs, lazyenv, ... }: {
    nixosConfigurations.myhost = nixpkgs.lib.nixosSystem {
      modules = [
        lazyenv.nixosModules.default
        { programs.lazyenv.enable = true; }
      ];
    };
  };
}
```

### Home Manager

Import the Home Manager module. `settings` is written to lazyenv's
`config.toml` (see [Configuration](configuration.md) for the keys):

```nix
{
  imports = [ inputs.lazyenv.homeManagerModules.default ];

  programs.lazyenv = {
    enable = true;
    settings = {
      sort = "alphabetical";
      layout.file-list-width = 40;
    };
  };
}
```

You can also skip the modules and add `lazyenv.packages.${system}.default`
to `environment.systemPackages` or `home.packages`, or apply
`lazyenv.overlays.default` to get `pkgs.lazyenv`.

## From source

Requires Go 1.26+.

```bash
go install github.com/lazynop/lazyenv@latest
```

## Build locally

```bash
just build        # build to bin/lazyenv
just run          # run
```

## Next steps

See the [Usage](usage.md) page for all CLI flags and options.
