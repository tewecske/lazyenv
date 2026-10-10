{
  description = "lazyenv - a TUI for managing .env files";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      lib = nixpkgs.lib;
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = f: lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      # Bump with each release tag.
      baseVersion = "0.9.0";
      date = builtins.substring 0 8 (self.lastModifiedDate or "19700101");
      commit = self.shortRev or self.dirtyShortRev or "unknown";
    in
    {
      packages = forAllSystems (pkgs: {
        default = self.packages.${pkgs.stdenv.hostPlatform.system}.lazyenv;
        lazyenv = pkgs.buildGoModule {
          pname = "lazyenv";
          version = "${baseVersion}-unstable-${date}";
          src = lib.cleanSource ./.;
          # Run `just flake-hash` after changing go.mod or go.sum; the pre-push hook checks this stamp.
          # vendorHash inputs (go.mod, go.sum): d0ba8daf219ad3aed13946f298bcd73cf344037d155fe297c651f37e48bd1d42
          vendorHash = "sha256-7G/7YVJ5JomnMKQNki90/DK/zVWvAW0PBEWGlmLl+AE=";

          subPackages = [ "." ];
          env.CGO_ENABLED = 0;
          ldflags = [
            "-s"
            "-w"
            "-X main.version=${baseVersion}-unstable-${date}"
            "-X main.commit=${commit}"
            "-X main.date=${date}"
          ];

          meta = {
            description = "TUI for managing .env files";
            homepage = "https://github.com/lazynop/lazyenv";
            license = lib.licenses.mit;
            mainProgram = "lazyenv";
          };
        };
      });

      overlays.default = final: _prev: {
        lazyenv = self.packages.${final.stdenv.hostPlatform.system}.lazyenv;
      };

      nixosModules.default =
        { config, pkgs, ... }:
        let
          cfg = config.programs.lazyenv;
        in
        {
          options.programs.lazyenv = {
            enable = lib.mkEnableOption "lazyenv";
            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.stdenv.hostPlatform.system}.lazyenv;
              description = "The lazyenv package to install.";
            };
          };
          config = lib.mkIf cfg.enable {
            environment.systemPackages = [ cfg.package ];
          };
        };

      homeManagerModules.default =
        { config, pkgs, ... }:
        let
          cfg = config.programs.lazyenv;
          tomlFormat = pkgs.formats.toml { };
          # Matches os.UserConfigDir() as used by internal/config/load.go.
          configPath =
            if pkgs.stdenv.hostPlatform.isDarwin then
              "Library/Application Support/lazyenv/config.toml"
            else
              "${lib.removePrefix "${config.home.homeDirectory}/" config.xdg.configHome}/lazyenv/config.toml";
        in
        {
          options.programs.lazyenv = {
            enable = lib.mkEnableOption "lazyenv";
            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.stdenv.hostPlatform.system}.lazyenv;
              description = "The lazyenv package to install.";
            };
            settings = lib.mkOption {
              type = tomlFormat.type;
              default = { };
              example = lib.literalExpression ''
                {
                  sort = "alphabetical";
                  layout.file-list-width = 40;
                  secrets.safe_patterns = [ "PUBLIC_" ];
                }
              '';
              description = ''
                Configuration written to lazyenv's config.toml.
                See docs/configuration.md for the available keys.
              '';
            };
          };
          config = lib.mkIf cfg.enable {
            home.packages = [ cfg.package ];
            home.file.${configPath} = lib.mkIf (cfg.settings != { }) {
              source = tomlFormat.generate "lazyenv-config.toml" cfg.settings;
            };
          };
        };

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            just
            git
          ];
        };
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt);
    };
}
